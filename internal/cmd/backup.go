package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/up1512001/memorybox/internal/app"
	"github.com/up1512001/memorybox/internal/gitignore"
	"github.com/up1512001/memorybox/internal/manifest"
	"github.com/up1512001/memorybox/internal/notify"
	"github.com/up1512001/memorybox/internal/rsync"
	"github.com/up1512001/memorybox/internal/scheduler"
	"github.com/up1512001/memorybox/internal/snapshot"
)

func newBackupCmd(a *app.App) *cobra.Command {
	var (
		message  string
		sections []string
		parallel int
		dryRun   bool
	)

	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Run a backup snapshot (default command)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runBackup(cmd.Context(), a, backupOpts{
				message:  message,
				sections: sections,
				parallel: parallel,
				dryRun:   dryRun,
			})
		},
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "snapshot label")
	cmd.Flags().StringSliceVarP(&sections, "sections", "s", nil,
		"sections to include (e.g. photos,docs,dev,config,icloud)")
	cmd.Flags().IntVar(&parallel, "parallel", 0, "max concurrent rsync sections (overrides config)")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "preview without making changes")

	return cmd
}

type backupOpts struct {
	message  string
	sections []string
	parallel int
	dryRun   bool
}

func runBackup(ctx context.Context, a *app.App, opts backupOpts) error {
	if err := a.CheckDrive(0); err != nil {
		return err
	}

	// Warn if system rsync is Apple's openrsync (broken --backup-dir in 15.4+).
	if warn := rsync.CheckVersion(""); warn != "" {
		a.Printer.Warn(warn)
	}

	// Ensure required directories exist.
	for _, dir := range []string{
		a.Cfg.Drive.BackupDir,
		a.Cfg.Drive.ArchiveDir,
		a.Cfg.Drive.ManifestDir,
		a.Cfg.Drive.LogDir,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}

	// Create the snapshot record (allocates key + archive dir).
	label := opts.message
	if label == "" {
		label = "Backup " + time.Now().UTC().Format("2006-01-02 15:04")
	}
	sectionNames := enabledSections(a, opts.sections)
	snap, err := a.Store.Create(ctx, snapshot.SnapshotMeta{
		Label:    label,
		Sections: sectionNames,
	})
	if err != nil {
		return fmt.Errorf("create snapshot: %w", err)
	}

	a.Printer.Info(fmt.Sprintf(`Snapshot %s — "%s"`, snap.Key, label))
	if opts.dryRun {
		a.Printer.Warn("dry-run: no changes will be made")
	}

	// Build the section job list.
	maxWorkers := a.Cfg.Parallel
	if opts.parallel > 0 {
		maxWorkers = opts.parallel
	}
	sched := scheduler.New(maxWorkers)

	var (
		totalChanged  int64
		totalArchived int64
		totalSent     int64
		totalFiles    int64
	)

	for i, name := range sectionNames {
		i, name := i, name
		sec, ok := a.Cfg.Sections[name]
		if !ok || !sec.Enabled {
			continue
		}

		sched.Submit(scheduler.Job{
			Name: name,
			Run: func(ctx context.Context) error {
				a.Printer.Section(i+1, len(sectionNames), name)

				// Pre-backup hook.
				if sec.Hooks.Pre != "" && !opts.dryRun {
					if err := runHook(ctx, sec.Hooks.Pre, name, "pre"); err != nil {
						a.Printer.Warn(fmt.Sprintf("%s pre-hook: %v", name, err))
					}
				}

				// Merge gitignore patterns into excludes if opted in.
				excludes := sec.Excludes
				if sec.GitignoreAware {
					excludes = mergeExcludes(excludes, gitignore.CollectExcludes(sec.Source, 4))
				}

				err := runSection(ctx, a, snap, sec.Source,
					filepath.Join(a.Cfg.Drive.BackupDir, sec.Dest),
					excludes, sec.Delete, opts.dryRun,
					&totalChanged, &totalArchived, &totalSent, &totalFiles)

				// Post-backup hook (runs even on error).
				if sec.Hooks.Post != "" && !opts.dryRun {
					if hookErr := runHook(ctx, sec.Hooks.Post, name, "post"); hookErr != nil {
						a.Printer.Warn(fmt.Sprintf("%s post-hook: %v", name, hookErr))
					}
				}

				return err
			},
		})
	}

	// Config/dotfiles section gets special treatment (assembled from many sources).
	if _, ok := a.Cfg.Sections["config"]; ok && containsSection(sectionNames, "config") {
		sched.Submit(scheduler.Job{
			Name: "config",
			Run: func(ctx context.Context) error {
				a.Printer.Section(len(sectionNames), len(sectionNames), "Config & Dotfiles")
				return runDotfiles(ctx, a, snap, opts.dryRun)
			},
		})
	}

	results := sched.Run(ctx)
	var sectionErrors []string
	for _, r := range results {
		if r.Err != nil {
			a.Printer.Warn(fmt.Sprintf("%s: %v", r.Name, r.Err))
			sectionErrors = append(sectionErrors, r.Name)
		}
	}

	// Generate manifest by walking the current backup directory.
	a.Printer.Info("Generating manifest…")
	if err := generateManifest(ctx, a, snap, label, totalChanged, totalArchived, totalSent); err != nil {
		a.Printer.Warn(fmt.Sprintf("manifest: %v", err))
	}

	// Summary.
	printSummary(a, snap, totalChanged, totalArchived, totalSent, totalFiles)

	// Append to history CSV.
	appendHistory(a, snap.Key, totalFiles, totalChanged, totalArchived, totalSent)

	// OS notification.
	if len(sectionErrors) > 0 {
		notify.Failure("Memory Box", fmt.Sprintf("Backup %s failed: %s", snap.Key, strings.Join(sectionErrors, ", ")))
		return fmt.Errorf("backup completed with errors in sections: %s", strings.Join(sectionErrors, ", "))
	}
	notify.Success("Memory Box", fmt.Sprintf("Snapshot %s — %d updated", snap.Key, totalChanged))

	return nil
}

func runSection(ctx context.Context, a *app.App, snap snapshot.Snapshot,
	src, dest string, excludes []string, delete, dryRun bool,
	changed, archived, sent, files *int64) error {

	if _, err := os.Stat(src); os.IsNotExist(err) {
		a.Printer.Warn(fmt.Sprintf("source not found, skipping: %s", src))
		return nil
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dest, err)
	}

	args := rsync.BuildArgs(rsync.SectionRunOpts{
		Source:      src,
		Destination: dest,
		ArchiveDir:  snap.ArchivePath,
		Excludes:    excludes,
		Delete:      delete,
		Verbose:     a.Cfg.UI.Verbose,
		DryRun:      dryRun,
	})

	lines := make(chan rsync.Line, 256)
	var stats *rsync.Stats
	var runErr error

	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		stats, runErr = a.Rsync.Run(ctx, args, lines)
	}()

	var sChanged, sArchived int64
	for line := range lines {
		if line.IsError {
			a.Printer.Warn(line.Raw)
			continue
		}
		if rsync.IsChanged(line.Raw) {
			sChanged++
		}
		if rsync.IsDeleted(line.Raw) {
			sArchived++
		}
	}
	<-doneCh

	if stats != nil {
		*sent += stats.TotalTransferred
		*files += stats.FilesTransferred
	}
	*changed += sChanged
	*archived += sArchived

	if sChanged == 0 && sArchived == 0 {
		a.Printer.Success(fmt.Sprintf("no changes"))
	} else {
		a.Printer.Success(fmt.Sprintf("%d updated, %d archived", sChanged, sArchived))
	}

	return runErr
}

func runDotfiles(ctx context.Context, a *app.App, snap snapshot.Snapshot, dryRun bool) error {
	dotfilesDir := filepath.Join(a.Cfg.Drive.BackupDir, "dotfiles")
	if err := os.MkdirAll(dotfilesDir, 0o755); err != nil {
		return err
	}

	home, _ := os.UserHomeDir()

	type copySpec struct{ src, dst string }
	copies := []copySpec{
		{home + "/.zshrc", dotfilesDir + "/zshrc"},
		{home + "/.zprofile", dotfilesDir + "/zprofile"},
		{home + "/.gitconfig", dotfilesDir + "/gitconfig"},
		{home + "/.npmrc", dotfilesDir + "/npmrc"},
	}

	type rsyncSpec struct{ src, dst string }
	rsyncs := []rsyncSpec{
		{home + "/.ssh", dotfilesDir + "/ssh"},
		{home + "/.gnupg", dotfilesDir + "/gnupg"},
		{home + "/.vscode", dotfilesDir + "/vscode"},
		{home + "/.claude", dotfilesDir + "/claude"},
		{home + "/.wp-cli", dotfilesDir + "/wp-cli"},
		{home + "/.composer", dotfilesDir + "/composer"},
		{home + "/.oh-my-zsh", dotfilesDir + "/oh-my-zsh"},
	}

	for _, c := range copies {
		if !dryRun {
			data, err := os.ReadFile(c.src)
			if err != nil {
				continue // file may not exist
			}
			os.WriteFile(c.dst, data, 0o600)
		}
	}

	for _, r := range rsyncs {
		if _, err := os.Stat(r.src); os.IsNotExist(err) {
			continue
		}
		args := rsync.BuildArgs(rsync.SectionRunOpts{
			Source:      r.src,
			Destination: r.dst,
			ArchiveDir:  snap.ArchivePath,
			DryRun:      dryRun,
		})
		lines := make(chan rsync.Line, 64)
		go func() { a.Rsync.Run(ctx, args, lines) }()
		for range lines {
		}
	}

	// Software inventory.
	invDir := filepath.Join(a.Cfg.Drive.BackupDir, "inventory")
	if !dryRun {
		os.MkdirAll(invDir, 0o755)
		runToFile("brew", []string{"list", "--versions"}, invDir+"/brew-formulae.txt")
		runToFile("brew", []string{"list", "--cask", "--versions"}, invDir+"/brew-casks.txt")
		runToFile("npm", []string{"list", "-g", "--depth=0"}, invDir+"/npm-global.txt")
		runToFile("composer", []string{"global", "show"}, invDir+"/composer-global.txt")
		runToFile("code", []string{"--list-extensions"}, invDir+"/vscode-extensions.txt")
		runToFileWithShell(
			`source "$HOME/.nvm/nvm.sh" 2>/dev/null && nvm ls --no-colors`,
			invDir+"/nvm-versions.txt",
		)
		// /etc/hosts
		if data, err := os.ReadFile("/etc/hosts"); err == nil {
			os.WriteFile(invDir+"/etc-hosts.txt", data, 0o644)
		}
	}

	a.Printer.Success("dotfiles and inventory synced")
	return nil
}

func generateManifest(ctx context.Context, a *app.App, snap snapshot.Snapshot,
	label string, updated, archived, sent int64) error {

	entries, err := manifest.Walk(ctx, a.Cfg.Drive.BackupDir)
	if err != nil {
		return err
	}

	w, closeW, err := a.Store.ManifestWriter(snap.Key)
	if err != nil {
		return err
	}
	defer closeW()

	if err := w.WriteHeader(manifest.Header{
		Message:     label,
		Snapshot:    snap.Key,
		Updated:     int(updated),
		Archived:    int(archived),
		Transferred: sent,
	}); err != nil {
		return err
	}

	for _, e := range entries {
		if err := w.Write(e); err != nil {
			return err
		}
	}

	// Populate BoltDB index for instant restore lookups.
	if a.Index != nil {
		i := 0
		indexErr := a.Index.IndexFromReader(snap.Key, func() (string, int64, time.Time, error) {
			if i >= len(entries) {
				return "", 0, time.Time{}, io.EOF
			}
			e := entries[i]
			i++
			return e.Path, e.Size, e.MTime, nil
		})
		if indexErr != nil {
			a.Printer.Warn(fmt.Sprintf("index: %v", indexErr))
		}
	}

	return nil
}

func printSummary(a *app.App, snap snapshot.Snapshot,
	changed, archived, sent, files int64) {

	a.Printer.Info("")
	a.Printer.Info(fmt.Sprintf("═══ Snapshot %s ═══", snap.Key))
	a.Printer.Info(fmt.Sprintf("  %-16s %d", "Updated:", changed))
	a.Printer.Info(fmt.Sprintf("  %-16s %d", "Archived:", archived))
	a.Printer.Info(fmt.Sprintf("  %-16s %s", "Transferred:", humanBytes(sent)))
	a.Printer.Info("")
	a.Printer.Info("Commands: membox log · membox diff · membox restore · membox status")
}

func appendHistory(a *app.App, key string, files, changed, archived, sent int64) {
	csvPath := filepath.Join(a.Cfg.Drive.LogDir, "history.csv")
	header := "snapshot,files,updated,archived,bytes\n"

	if _, err := os.Stat(csvPath); os.IsNotExist(err) {
		os.WriteFile(csvPath, []byte(header), 0o644)
	}

	f, err := os.OpenFile(csvPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s,%d,%d,%d,%d\n", key, files, changed, archived, sent)
}

func enabledSections(a *app.App, requested []string) []string {
	if len(requested) > 0 {
		var out []string
		for _, name := range requested {
			if sec, ok := a.Cfg.Sections[name]; ok && sec.Enabled && name != "config" {
				out = append(out, name)
			}
		}
		return out
	}

	// Built-in sections in canonical order, then any custom sections alphabetically.
	builtIn := []string{"photos", "movies", "docs", "desktop", "downloads", "icloud", "dev", "localsites"}
	seen := make(map[string]bool, len(builtIn)+1)
	seen["config"] = true

	var out []string
	for _, name := range builtIn {
		seen[name] = true
		if sec, ok := a.Cfg.Sections[name]; ok && sec.Enabled {
			out = append(out, name)
		}
	}

	var custom []string
	for name := range a.Cfg.Sections {
		if !seen[name] {
			custom = append(custom, name)
		}
	}
	sort.Strings(custom)
	for _, name := range custom {
		if sec := a.Cfg.Sections[name]; sec.Enabled {
			out = append(out, name)
		}
	}
	return out
}

func containsSection(sections []string, name string) bool {
	for _, s := range sections {
		if strings.EqualFold(s, name) {
			return true
		}
	}
	return false
}

func humanBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1fG", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1fK", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%dB", b)
	}
}

// runHook executes a shell command string via sh -c, inheriting the current
// environment. stderr goes to the terminal; a non-zero exit is returned as error.
func runHook(ctx context.Context, command, section, phase string) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s-hook exited: %w", section, phase, err)
	}
	return nil
}

// mergeExcludes appends extra patterns that aren't already in base.
func mergeExcludes(base, extra []string) []string {
	existing := make(map[string]bool, len(base))
	for _, p := range base {
		existing[p] = true
	}
	out := make([]string, len(base))
	copy(out, base)
	for _, p := range extra {
		if !existing[p] {
			out = append(out, p)
			existing[p] = true
		}
	}
	return out
}
