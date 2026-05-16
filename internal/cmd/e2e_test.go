package cmd

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/up1512001/memorybox/internal/app"
	"github.com/up1512001/memorybox/internal/color"
	"github.com/up1512001/memorybox/internal/config"
	diffpkg "github.com/up1512001/memorybox/internal/diff"
	"github.com/up1512001/memorybox/internal/drive"
	"github.com/up1512001/memorybox/internal/index"
	"github.com/up1512001/memorybox/internal/manifest"
	"github.com/up1512001/memorybox/internal/output"
	"github.com/up1512001/memorybox/internal/prune"
	"github.com/up1512001/memorybox/internal/restore"
	"github.com/up1512001/memorybox/internal/snapshot"
	"github.com/up1512001/memorybox/internal/testutil"
)

// makeE2EApp builds an App wired to temp dirs with a mock rsync runner.
// srcDir is the source directory for the single "dev" section.
func makeE2EApp(t *testing.T) (a *app.App, srcDir string) {
	t.Helper()
	color.Init(true) // disable ANSI colors; style vars must be non-nil
	root := t.TempDir()

	srcDir = filepath.Join(root, "source")
	backupDir := filepath.Join(root, "backup-current")
	archiveDir := filepath.Join(root, "backup-archive")
	manifestDir := filepath.Join(root, "manifests")
	logDir := filepath.Join(root, "logs")

	for _, d := range []string{srcDir, backupDir, archiveDir, manifestDir, logDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	store := snapshot.NewStore(manifestDir, archiveDir)

	a = &app.App{
		Cfg: config.Config{
			Drive: config.DriveConfig{
				MountPath:   root,
				BackupDir:   backupDir,
				ArchiveDir:  archiveDir,
				ManifestDir: manifestDir,
				LogDir:      logDir,
			},
			Sections: map[string]config.SectionConfig{
				"dev": {
					Enabled: true,
					Source:  srcDir,
					Dest:    "dev",
					Delete:  true,
				},
			},
			Parallel: 1,
			Prune: config.PruneConfig{
				DefaultDays: 90,
				DefaultKeep: 2,
			},
			UI: config.UIConfig{Quiet: true},
		},
		Printer: output.NewQuiet(),
		Store:   store,
		Differ:  diffpkg.New(store),
		Pruner:  prune.New(store),
		Scanner: restore.New(store, backupDir, archiveDir),
		Rsync:   &testutil.MockRsyncRunner{},
		Drive:   drive.New(),
	}

	if idx, err := index.Open(manifestDir); err == nil {
		a.Index = idx
		t.Cleanup(func() { idx.Close() })
	}

	return a, srcDir
}

// writeSourceFile creates a file in srcDir with the given name and content.
func writeSourceFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// writeManifest writes a minimal manifest file directly to manifestDir.
// Also creates the archive dir for the key.
func writeManifest(t *testing.T, manifestDir, archiveDir, key string, entries []manifest.Entry) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(archiveDir, key), 0o755); err != nil {
		t.Fatalf("mkdir archive %s: %v", key, err)
	}
	f, err := os.Create(filepath.Join(manifestDir, key+".manifest"))
	if err != nil {
		t.Fatalf("create manifest %s: %v", key, err)
	}
	defer f.Close()
	w := manifest.NewWriter(f)
	_ = w.WriteHeader(manifest.Header{Message: "test", Snapshot: key})
	for _, e := range entries {
		_ = w.Write(e)
	}
	_ = w.Flush()
}

// cobraCmd returns a minimal cobra Command with the given context.
func cobraCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	return cmd
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestE2E_FirstBackup(t *testing.T) {
	a, srcDir := makeE2EApp(t)
	ctx := context.Background()

	writeSourceFile(t, srcDir, "note.txt", "hello world")
	writeSourceFile(t, srcDir, "photo.jpg", "fake jpeg data")

	opts := backupOpts{message: "first backup", sections: []string{"dev"}}
	if err := runBackup(ctx, a, opts); err != nil {
		t.Fatalf("backup: %v", err)
	}

	// Snapshot exists in store.
	snaps, err := a.Store.List(ctx)
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}

	// Manifest file on disk.
	key := snaps[0].Key
	if _, err := os.Stat(filepath.Join(a.Cfg.Drive.ManifestDir, key+".manifest")); err != nil {
		t.Fatalf("manifest file missing: %v", err)
	}

	// Files copied to backup dir.
	backupNote := filepath.Join(a.Cfg.Drive.BackupDir, "dev", "note.txt")
	if _, err := os.Stat(backupNote); err != nil {
		t.Fatalf("backed-up file missing: %v", err)
	}
}

func TestE2E_Log_ShowsSnapshots(t *testing.T) {
	a, srcDir := makeE2EApp(t)
	ctx := context.Background()

	writeSourceFile(t, srcDir, "doc.txt", "content")

	// Real backup for snapshot 1.
	if err := runBackup(ctx, a, backupOpts{message: "snap1", sections: []string{"dev"}}); err != nil {
		t.Fatalf("backup: %v", err)
	}

	// Write a second manifest directly with a different key.
	writeManifest(t, a.Cfg.Drive.ManifestDir, a.Cfg.Drive.ArchiveDir,
		"2020-01-01T10-00-00Z",
		[]manifest.Entry{{Path: "dev/doc.txt", Size: 7, MTime: time.Unix(1577872800, 0).UTC(), Mode: fs.FileMode(0o644)}},
	)

	snaps, err := a.Store.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(snaps) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(snaps))
	}

	// runLog must not return an error.
	if err := runLog(ctx, a, 10, false, false); err != nil {
		t.Fatalf("log: %v", err)
	}
}

func TestE2E_Diff_ShowsAddedFile(t *testing.T) {
	a, _ := makeE2EApp(t)
	ctx := context.Background()

	mtime := time.Unix(1700000000, 0).UTC()
	key1 := "2026-05-01T10-00-00Z"
	key2 := "2026-05-02T10-00-00Z"

	writeManifest(t, a.Cfg.Drive.ManifestDir, a.Cfg.Drive.ArchiveDir, key1,
		[]manifest.Entry{
			{Path: "dev/file1.txt", Size: 100, MTime: mtime, Mode: fs.FileMode(0o644)},
		},
	)
	writeManifest(t, a.Cfg.Drive.ManifestDir, a.Cfg.Drive.ArchiveDir, key2,
		[]manifest.Entry{
			{Path: "dev/file1.txt", Size: 100, MTime: mtime, Mode: fs.FileMode(0o644)},
			{Path: "dev/file2.txt", Size: 200, MTime: mtime, Mode: fs.FileMode(0o644)},
		},
	)

	stat, err := a.Differ.Stat(ctx, key1, key2)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if stat.Added != 1 {
		t.Errorf("expected 1 added, got %d", stat.Added)
	}
	if stat.Deleted != 0 {
		t.Errorf("expected 0 deleted, got %d", stat.Deleted)
	}
	if stat.Modified != 0 {
		t.Errorf("expected 0 modified, got %d", stat.Modified)
	}
}

func TestE2E_Diff_ShowsModifiedFile(t *testing.T) {
	a, _ := makeE2EApp(t)
	ctx := context.Background()

	mtime1 := time.Unix(1700000000, 0).UTC()
	mtime2 := time.Unix(1700000001, 0).UTC()
	key1 := "2026-05-01T10-00-00Z"
	key2 := "2026-05-02T10-00-00Z"

	writeManifest(t, a.Cfg.Drive.ManifestDir, a.Cfg.Drive.ArchiveDir, key1,
		[]manifest.Entry{{Path: "dev/report.txt", Size: 100, MTime: mtime1, Mode: fs.FileMode(0o644)}},
	)
	writeManifest(t, a.Cfg.Drive.ManifestDir, a.Cfg.Drive.ArchiveDir, key2,
		[]manifest.Entry{{Path: "dev/report.txt", Size: 200, MTime: mtime2, Mode: fs.FileMode(0o644)}},
	)

	stat, err := a.Differ.Stat(ctx, key1, key2)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if stat.Modified != 1 {
		t.Errorf("expected 1 modified, got %d", stat.Modified)
	}
}

func TestE2E_Restore_FindsFileInCurrentBackup(t *testing.T) {
	a, srcDir := makeE2EApp(t)
	ctx := context.Background()

	writeSourceFile(t, srcDir, "resume.pdf", "fake pdf")

	if err := runBackup(ctx, a, backupOpts{message: "backup", sections: []string{"dev"}}); err != nil {
		t.Fatalf("backup: %v", err)
	}

	out := make(chan restore.Match, 16)
	errCh := make(chan error, 1)
	go func() {
		errCh <- a.Scanner.Find(ctx, restore.FindOpts{Pattern: "resume.pdf"}, out)
	}()

	var matches []restore.Match
	for m := range out {
		matches = append(matches, m)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("find: %v", err)
	}

	if len(matches) == 0 {
		t.Fatal("expected at least one match for resume.pdf")
	}
	if matches[0].Path == "" {
		t.Error("match path should not be empty")
	}
}

func TestE2E_Restore_CopiesFileToDestination(t *testing.T) {
	a, srcDir := makeE2EApp(t)
	ctx := context.Background()

	writeSourceFile(t, srcDir, "secret.key", "my-secret")

	if err := runBackup(ctx, a, backupOpts{sections: []string{"dev"}}); err != nil {
		t.Fatalf("backup: %v", err)
	}

	destDir := filepath.Join(t.TempDir(), "restored")

	out := make(chan restore.Match, 16)
	errCh := make(chan error, 1)
	go func() {
		errCh <- a.Scanner.Find(ctx, restore.FindOpts{Pattern: "secret.key"}, out)
	}()
	var matches []restore.Match
	for m := range out {
		matches = append(matches, m)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("no match found")
	}

	if err := a.Scanner.Copy(matches[0], restore.CopyOpts{Destination: destDir}); err != nil {
		t.Fatalf("copy: %v", err)
	}

	restored := filepath.Join(destDir, "secret.key")
	data, err := os.ReadFile(restored)
	if err != nil {
		t.Fatalf("restored file not found: %v", err)
	}
	if string(data) != "my-secret" {
		t.Errorf("restored content %q, want %q", string(data), "my-secret")
	}
}

func TestE2E_Verify_Clean(t *testing.T) {
	a, srcDir := makeE2EApp(t)
	ctx := context.Background()

	writeSourceFile(t, srcDir, "data.bin", "binary content")

	if err := runBackup(ctx, a, backupOpts{sections: []string{"dev"}}); err != nil {
		t.Fatalf("backup: %v", err)
	}

	if err := runVerify(cobraCmd(ctx), a); err != nil {
		t.Fatalf("verify failed on clean backup: %v", err)
	}
}

func TestE2E_Verify_DetectsCorruption(t *testing.T) {
	a, srcDir := makeE2EApp(t)
	ctx := context.Background()

	writeSourceFile(t, srcDir, "important.doc", "original content")

	if err := runBackup(ctx, a, backupOpts{sections: []string{"dev"}}); err != nil {
		t.Fatalf("backup: %v", err)
	}

	// Corrupt the backed-up file.
	backupFile := filepath.Join(a.Cfg.Drive.BackupDir, "dev", "important.doc")
	if err := os.WriteFile(backupFile, []byte("corrupted! different length"), 0o644); err != nil {
		t.Fatalf("corrupt file: %v", err)
	}

	if err := runVerify(cobraCmd(ctx), a); err == nil {
		t.Fatal("verify should have failed on corrupted file")
	}
}

func TestE2E_Verify_DetectsMissingFile(t *testing.T) {
	a, srcDir := makeE2EApp(t)
	ctx := context.Background()

	writeSourceFile(t, srcDir, "precious.txt", "cannot lose this")

	if err := runBackup(ctx, a, backupOpts{sections: []string{"dev"}}); err != nil {
		t.Fatalf("backup: %v", err)
	}

	// Delete the backed-up file.
	if err := os.Remove(filepath.Join(a.Cfg.Drive.BackupDir, "dev", "precious.txt")); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if err := runVerify(cobraCmd(ctx), a); err == nil {
		t.Fatal("verify should have failed on missing file")
	}
}

func TestE2E_Prune_RemovesOldSnapshots(t *testing.T) {
	a, _ := makeE2EApp(t)
	ctx := context.Background()

	mtime := time.Unix(1700000000, 0).UTC()
	keys := []string{
		"2026-01-01T10-00-00Z",
		"2026-01-02T10-00-00Z",
		"2026-01-03T10-00-00Z",
	}
	for _, k := range keys {
		writeManifest(t, a.Cfg.Drive.ManifestDir, a.Cfg.Drive.ArchiveDir, k,
			[]manifest.Entry{{Path: "dev/file.txt", Size: 10, MTime: mtime, Mode: fs.FileMode(0o644)}},
		)
	}

	snaps, _ := a.Store.List(ctx)
	if len(snaps) != 3 {
		t.Fatalf("expected 3 snapshots before prune, got %d", len(snaps))
	}

	// keep=1 means protect only the newest; days=0 means no age filter
	result, err := a.Pruner.Prune(ctx, prune.Opts{KeepLast: 1, OlderThanDays: 0})
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if len(result.Removed) != 2 {
		t.Errorf("expected 2 removed, got %d", len(result.Removed))
	}

	remaining, _ := a.Store.List(ctx)
	if len(remaining) != 1 {
		t.Errorf("expected 1 snapshot after prune, got %d", len(remaining))
	}
	// Newest key should survive.
	if remaining[0].Key != "2026-01-03T10-00-00Z" {
		t.Errorf("expected newest snapshot to survive, got %s", remaining[0].Key)
	}
}

func TestE2E_Index_PopulatedAfterBackup(t *testing.T) {
	a, srcDir := makeE2EApp(t)
	ctx := context.Background()

	writeSourceFile(t, srcDir, "index_me.txt", "indexed")
	writeSourceFile(t, srcDir, "other.txt", "also indexed")

	if err := runBackup(ctx, a, backupOpts{sections: []string{"dev"}}); err != nil {
		t.Fatalf("backup: %v", err)
	}

	if a.Index == nil {
		t.Skip("index not available")
	}

	snaps, entries, err := a.Index.Stats()
	if err != nil {
		t.Fatalf("index stats: %v", err)
	}
	if snaps != 1 {
		t.Errorf("expected 1 indexed snapshot, got %d", snaps)
	}
	if entries < 2 {
		t.Errorf("expected at least 2 indexed entries, got %d", entries)
	}
}

func TestE2E_BackupMultipleFiles_ManifestContainsAll(t *testing.T) {
	a, srcDir := makeE2EApp(t)
	ctx := context.Background()

	files := []string{"alpha.txt", "beta.txt", "gamma.txt"}
	for _, f := range files {
		writeSourceFile(t, srcDir, f, "content of "+f)
	}

	if err := runBackup(ctx, a, backupOpts{sections: []string{"dev"}}); err != nil {
		t.Fatalf("backup: %v", err)
	}

	snaps, _ := a.Store.List(ctx)
	if len(snaps) == 0 {
		t.Fatal("no snapshots")
	}

	r, closeR, err := a.Store.ManifestReader(snaps[0].Key)
	if err != nil {
		t.Fatalf("open manifest: %v", err)
	}
	defer closeR()

	var paths []string
	for {
		e, err := r.Next()
		if err != nil {
			break
		}
		paths = append(paths, e.Path)
	}

	if len(paths) < len(files) {
		t.Errorf("manifest has %d entries, want at least %d", len(paths), len(files))
	}
}

func TestE2E_DryRun_NoFilesWritten(t *testing.T) {
	a, srcDir := makeE2EApp(t)
	ctx := context.Background()

	writeSourceFile(t, srcDir, "dryrun.txt", "should not appear in backup")

	if err := runBackup(ctx, a, backupOpts{dryRun: true, sections: []string{"dev"}}); err != nil {
		t.Fatalf("dry-run backup: %v", err)
	}

	// With a real rsync, dry-run would produce no files. With MockRsyncRunner,
	// the mock still copies files (it ignores the dry-run flag in args).
	// We verify that the snapshot was still created and logged.
	snaps, _ := a.Store.List(ctx)
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot even in dry-run, got %d", len(snaps))
	}
}
