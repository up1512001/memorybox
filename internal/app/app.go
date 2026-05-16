// Package app wires all subsystems together and owns the cobra root command.
package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/up1512001/memorybox/internal/color"
	"github.com/up1512001/memorybox/internal/config"
	"github.com/up1512001/memorybox/internal/diff"
	"github.com/up1512001/memorybox/internal/drive"
	"github.com/up1512001/memorybox/internal/index"
	"github.com/up1512001/memorybox/internal/output"
	"github.com/up1512001/memorybox/internal/prune"
	"github.com/up1512001/memorybox/internal/restore"
	"github.com/up1512001/memorybox/internal/rsync"
	"github.com/up1512001/memorybox/internal/snapshot"
)

// BuildInfo holds version metadata injected via ldflags.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

// App holds all wired-up subsystems.
type App struct {
	Build   BuildInfo
	V       *viper.Viper
	Cfg     config.Config
	Printer output.Printer
	Store   *snapshot.Store
	Differ  *diff.Differ
	Pruner  *prune.Pruner
	Scanner *restore.Scanner
	Rsync   rsync.Runner
	Drive   *drive.Prober
	Index   *index.Index
}

// New creates an App with all subsystems lazy-initialised after flag parsing.
func New(build BuildInfo) *App {
	return &App{
		Build: build,
		V:     viper.New(),
		Rsync: &rsync.Exec{},
		Drive: drive.New(),
	}
}

// Init wires all subsystems from the loaded config.
// Must be called after flags are parsed (in PersistentPreRunE).
func (a *App) Init(cfgFile string, noColor bool) error {
	cfg, err := config.Load(a.V, cfgFile)
	if err != nil {
		return err
	}
	a.Cfg = cfg

	// Color must be initialised before any output.
	color.Init(noColor || !cfg.UI.Color)

	if cfg.UI.Quiet {
		a.Printer = output.NewQuiet()
	} else {
		a.Printer = output.NewTTY(os.Stdout)
	}

	a.Store = snapshot.NewStore(cfg.Drive.ManifestDir, cfg.Drive.ArchiveDir)
	a.Differ = diff.New(a.Store)
	a.Pruner = prune.New(a.Store)
	a.Scanner = restore.New(a.Store, cfg.Drive.BackupDir, cfg.Drive.ArchiveDir)

	// Index is optional — open if the manifest dir exists, skip silently otherwise.
	if cfg.Drive.ManifestDir != "" {
		if idx, err := index.Open(cfg.Drive.ManifestDir); err == nil {
			a.Index = idx
		}
	}
	return nil
}

// CheckDrive verifies the backup drive is mounted and has sufficient space.
func (a *App) CheckDrive(required int64) error {
	if _, err := os.Stat(a.Cfg.Drive.MountPath); os.IsNotExist(err) {
		msg := fmt.Sprintf("drive not found at %q", a.Cfg.Drive.MountPath)
		if vols := config.AvailableVolumes(); len(vols) > 0 {
			msg += fmt.Sprintf("\n  Available volumes: %s", strings.Join(vols, ", "))
			msg += "\n  Use --drive to override or run `membox init` to reconfigure"
		} else {
			msg += "\n  Connect your SSD and try again, or run `membox init` to configure"
		}
		return fmt.Errorf("%s", msg)
	}
	if required > 0 {
		ok, err := a.Drive.HasSpace(nil, a.Cfg.Drive.MountPath, required)
		if err == nil && !ok {
			info, _ := a.Drive.Probe(nil, a.Cfg.Drive.MountPath)
			a.Printer.Warn(fmt.Sprintf("low disk space: only %s free",
				humanBytes(info.FreeBytes)))
		}
	}
	return nil
}

// RootCmd builds the root cobra command with all subcommands attached.
func (a *App) RootCmd() *cobra.Command {
	return nil // filled in by internal/cmd/root.go
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
