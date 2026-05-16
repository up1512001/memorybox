package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/up1512001/memorybox/internal/app"
	"github.com/up1512001/memorybox/internal/config"
	"github.com/up1512001/memorybox/internal/drive"
)

func newInitCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Interactive setup — pick drive, configure sections, write config",
		Long: `Walk through initial Memory Box setup:
  1. Pick which external drive to back up to
  2. Choose which sections (folders) to back up
  3. Configure exclude patterns per section
  4. Add custom folders (any path on your Mac)
  5. Write ~/.config/memorybox/config.yaml`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInit(a, os.Stdin, os.Stdout)
		},
	}
}

func runInit(a *app.App, stdin *os.File, stdout *os.File) error {
	in := bufio.NewReader(stdin)

	printWizardBanner(stdout)

	mountPath, err := wizardPickDrive(in, stdout)
	if err != nil {
		return err
	}

	sections, err := wizardPickSections(in, stdout, a.Cfg.Sections)
	if err != nil {
		return err
	}

	sections = wizardConfigureExcludes(in, stdout, sections)
	sections = wizardAddCustomFolders(in, stdout, sections)

	cfg := a.Cfg
	cfg.Drive.MountPath = mountPath
	cfg.Drive.BackupDir = mountPath + "/backup-current"
	cfg.Drive.ArchiveDir = mountPath + "/backup-archive"
	cfg.Drive.ManifestDir = mountPath + "/backup-manifests"
	cfg.Drive.LogDir = mountPath + "/backup-logs"
	cfg.Sections = sections

	cfgPath := defaultConfigPath()

	printConfigPreview(stdout, cfg, cfgPath)

	if !wizardConfirmWrite(in, stdout, cfgPath) {
		fmt.Fprintln(stdout, "Aborted — nothing written.")
		return nil
	}

	if err := config.WriteConfig(cfgPath, cfg); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	fmt.Fprintf(stdout, "\n✓  Config written to %s\n", cfgPath)
	fmt.Fprintf(stdout, "   Run `membox` to start your first backup.\n\n")
	return nil
}

// ── wizard steps ──────────────────────────────────────────────────────────────

func printWizardBanner(w io.Writer) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Memory Box Setup")
	fmt.Fprintln(w, strings.Repeat("─", 52))
	fmt.Fprintln(w)
}

func wizardPickDrive(in *bufio.Reader, w io.Writer) (string, error) {
	fmt.Fprintln(w, "Step 1 of 4  —  Backup drive")
	fmt.Fprintln(w)

	vols := config.AvailableVolumes()
	prober := drive.New()

	for i, vol := range vols {
		info, err := prober.Probe(nil, vol)
		if err != nil {
			fmt.Fprintf(w, "  [%d] %s\n", i+1, vol)
		} else {
			pct := int(100 * info.UsedBytes / (info.TotalBytes + 1))
			fmt.Fprintf(w, "  [%d] %-30s  %s free of %s  (%d%% used)\n",
				i+1, vol,
				humanBytes(info.FreeBytes),
				humanBytes(info.TotalBytes),
				pct,
			)
		}
	}
	fmt.Fprintf(w, "  [0] Enter a custom path\n\n")

	defIdx := 1
	if len(vols) == 0 {
		defIdx = 0
	}
	fmt.Fprintf(w, "Pick drive [%d]: ", defIdx)

	line := readLine(in)
	if line == "" {
		line = fmt.Sprintf("%d", defIdx)
	}

	if line == "0" || len(vols) == 0 {
		fmt.Fprintf(w, "Drive mount path: ")
		return readLine(in), nil
	}

	var idx int
	if _, err := fmt.Sscanf(line, "%d", &idx); err != nil || idx < 1 || idx > len(vols) {
		return "", fmt.Errorf("invalid selection %q — run `membox init` again", line)
	}
	return vols[idx-1], nil
}

func wizardPickSections(in *bufio.Reader, w io.Writer, current map[string]config.SectionConfig) (map[string]config.SectionConfig, error) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Step 2 of 4  —  Sections to back up")
	fmt.Fprintln(w)

	order := wizardSectionOrder()
	printSectionTable(w, order, current)

	fmt.Fprintln(w)
	fmt.Fprintf(w, "Disable sections (comma-separated names, or Enter to keep all):\n> ")

	line := readLine(in)

	result := cloneSections(current)

	if line == "" {
		return result, nil
	}

	for _, name := range strings.Split(line, ",") {
		name = strings.TrimSpace(strings.ToLower(name))
		if sec, ok := result[name]; ok {
			sec.Enabled = false
			result[name] = sec
		}
	}

	fmt.Fprintln(w)
	printSectionTable(w, order, result)
	return result, nil
}

func wizardConfigureExcludes(in *bufio.Reader, w io.Writer, sections map[string]config.SectionConfig) map[string]config.SectionConfig {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Step 3 of 4  —  Exclude patterns")
	fmt.Fprintln(w)

	result := cloneSections(sections)

	anyShown := false
	for _, name := range wizardSectionOrder() {
		sec, ok := result[name]
		if !ok || !sec.Enabled || len(sec.Excludes) == 0 {
			continue
		}
		anyShown = true
		fmt.Fprintf(w, "  %-12s  current: %s\n", name, strings.Join(sec.Excludes, ", "))
		fmt.Fprintf(w, "              add more (comma-separated, or Enter to keep): ")
		line := readLine(in)
		if line == "" {
			continue
		}
		for _, p := range strings.Split(line, ",") {
			if p = strings.TrimSpace(p); p != "" {
				sec.Excludes = append(sec.Excludes, p)
			}
		}
		result[name] = sec
		fmt.Fprintln(w)
	}

	if !anyShown {
		fmt.Fprintln(w, "  No sections with default excludes are enabled — skipped.")
	}

	return result
}

func printConfigPreview(w io.Writer, cfg config.Config, cfgPath string) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Config preview:")
	fmt.Fprintf(w, "  Path:     %s\n", cfgPath)
	fmt.Fprintf(w, "  Drive:    %s\n", cfg.Drive.MountPath)

	builtIn := wizardSectionOrder()
	builtInSet := make(map[string]bool, len(builtIn))
	for _, n := range builtIn {
		builtInSet[n] = true
	}

	var enabled, custom []string
	for _, name := range builtIn {
		if s, ok := cfg.Sections[name]; ok && s.Enabled {
			enabled = append(enabled, name)
		}
	}
	for name, s := range cfg.Sections {
		if !builtInSet[name] && s.Enabled {
			custom = append(custom, name)
		}
	}
	fmt.Fprintf(w, "  Sections: %s\n", strings.Join(enabled, ", "))
	if len(custom) > 0 {
		fmt.Fprintf(w, "  Custom:   %s\n", strings.Join(custom, ", "))
	}
	fmt.Fprintln(w)
}

func wizardAddCustomFolders(in *bufio.Reader, w io.Writer, sections map[string]config.SectionConfig) map[string]config.SectionConfig {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Step 4 of 4  —  Custom folders")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Back up any folder not in the list above.")
	fmt.Fprintln(w, "  Example: ~/Work/Projects → section name \"work\"")
	fmt.Fprintln(w)

	result := cloneSections(sections)

	for {
		fmt.Fprintf(w, "Add a custom folder? [y/N]: ")
		ans := strings.ToLower(readLine(in))
		if ans != "y" && ans != "yes" {
			break
		}

		fmt.Fprintf(w, "  Source path (e.g. ~/Projects): ")
		src := readLine(in)
		if src == "" {
			fmt.Fprintln(w, "  Source path cannot be empty — skipped.")
			continue
		}

		// Expand ~ manually for display (config.Load does full expansion at runtime)
		if strings.HasPrefix(src, "~/") {
			home, _ := os.UserHomeDir()
			src = home + src[1:]
		}

		fmt.Fprintf(w, "  Section name (letters/numbers/hyphens, used as folder on drive): ")
		name := strings.ToLower(strings.TrimSpace(readLine(in)))
		if name == "" || name == "config" {
			fmt.Fprintln(w, "  Invalid name — skipped.")
			continue
		}
		if _, exists := result[name]; exists {
			fmt.Fprintf(w, "  Section %q already exists — skipped.\n", name)
			continue
		}

		fmt.Fprintf(w, "  Exclude patterns (comma-separated, or Enter for none): ")
		var excludes []string
		if line := readLine(in); line != "" {
			for _, p := range strings.Split(line, ",") {
				if p = strings.TrimSpace(p); p != "" {
					excludes = append(excludes, p)
				}
			}
		}

		result[name] = config.SectionConfig{
			Enabled:  true,
			Source:   src,
			Dest:     name,
			Excludes: excludes,
			Delete:   true,
		}
		fmt.Fprintf(w, "  ✓ Added section %q → %s\n\n", name, src)
	}

	return result
}

func wizardConfirmWrite(in *bufio.Reader, w io.Writer, cfgPath string) bool {
	if _, err := os.Stat(cfgPath); err == nil {
		fmt.Fprintf(w, "Config already exists at %s\nOverwrite? [y/N]: ", cfgPath)
		return strings.ToLower(readLine(in)) == "y"
	}
	fmt.Fprintf(w, "Write config? [Y/n]: ")
	ans := strings.ToLower(readLine(in))
	return ans == "" || ans == "y" || ans == "yes"
}

// ── helpers ───────────────────────────────────────────────────────────────────

func printSectionTable(w io.Writer, order []string, sections map[string]config.SectionConfig) {
	for _, name := range order {
		sec, ok := sections[name]
		if !ok {
			continue
		}
		mark := "✓"
		if !sec.Enabled {
			mark = "✗"
		}
		src := sec.Source
		if src == "" {
			src = "dotfiles + inventory"
		}
		fmt.Fprintf(w, "  [%s] %-12s  %s\n", mark, name, src)
	}
}

func cloneSections(src map[string]config.SectionConfig) map[string]config.SectionConfig {
	out := make(map[string]config.SectionConfig, len(src))
	for k, v := range src {
		if len(v.Excludes) > 0 {
			ex := make([]string, len(v.Excludes))
			copy(ex, v.Excludes)
			v.Excludes = ex
		}
		out[k] = v
	}
	return out
}

func wizardSectionOrder() []string {
	return []string{"photos", "movies", "docs", "desktop", "downloads", "icloud", "dev", "localsites", "config"}
}

func defaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "memorybox", "config.yaml")
}

func readLine(in *bufio.Reader) string {
	line, _ := in.ReadString('\n')
	return strings.TrimSpace(line)
}
