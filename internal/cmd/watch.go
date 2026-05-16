package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/up1512001/memorybox/internal/app"
	"github.com/up1512001/memorybox/internal/notify"
	"github.com/up1512001/memorybox/internal/watcher"
)

func newWatchCmd(a *app.App) *cobra.Command {
	var debounce time.Duration

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Backup automatically when the drive is connected",
		Long: `Watches for the configured backup drive to appear and runs membox backup
automatically. Press Ctrl-C to stop watching.

On macOS, monitors /Volumes. On Linux, monitors /media/$USER and /run/media/$USER.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runWatch(cmd.Context(), a, debounce)
		},
	}

	cmd.Flags().DurationVar(&debounce, "debounce", 5*time.Second,
		"wait this long after drive appears before starting backup (allows drive to fully mount)")

	return cmd
}

func runWatch(ctx context.Context, a *app.App, debounce time.Duration) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	defer w.Close()

	dirs := watcher.WatchDirs()
	watched := 0
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err == nil {
			if err := w.Add(dir); err == nil {
				watched++
			}
		}
	}
	if watched == 0 {
		return fmt.Errorf("could not watch any mount directories: %v", dirs)
	}

	target := a.Cfg.Drive.MountPath
	a.Printer.Info(fmt.Sprintf("Watching for drive at %s — press Ctrl-C to stop", target))
	a.Printer.Info(fmt.Sprintf("  Monitoring: %s", strings.Join(dirs, ", ")))

	// Handle Ctrl-C gracefully.
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var pendingTimer *time.Timer

	for {
		select {
		case <-ctx.Done():
			a.Printer.Info("Watch stopped.")
			return nil

		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			a.Printer.Warn(fmt.Sprintf("watcher error: %v", err))

		case event, ok := <-w.Events:
			if !ok {
				return nil
			}
			if !watcher.IsMountEvent(event) {
				continue
			}
			mountPath := watcher.MountPath(event)
			if !strings.HasPrefix(target, mountPath) && mountPath != target {
				// A different drive appeared — not our target.
				a.Printer.Info(fmt.Sprintf("Drive appeared: %s (not the configured backup drive, skipping)", mountPath))
				continue
			}

			a.Printer.Info(fmt.Sprintf("Backup drive detected: %s — backup starts in %s", mountPath, debounce))
			notify.Success("Memory Box", fmt.Sprintf("Drive connected — backup starting in %s", debounce))

			// Debounce: reset timer if drive keeps sending events.
			if pendingTimer != nil {
				pendingTimer.Stop()
			}
			pendingTimer = time.AfterFunc(debounce, func() {
				a.Printer.Info("Starting automatic backup…")
				if err := runBackup(ctx, a, backupOpts{}); err != nil {
					a.Printer.Warn(fmt.Sprintf("auto-backup error: %v", err))
					notify.Failure("Memory Box", fmt.Sprintf("Auto-backup failed: %v", err))
				}
			})
		}
	}
}
