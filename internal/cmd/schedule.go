package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/up1512001/memorybox/internal/app"
)

func newScheduleCmd(a *app.App) *cobra.Command {
	var (
		interval string
		bwlimit  int
		nice     bool
		remove   bool
	)

	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Install (or remove) a scheduled backup job",
		Long: `On macOS: writes a LaunchAgent plist to ~/Library/LaunchAgents and loads it.
On Linux: writes a systemd user service + timer to ~/.config/systemd/user/.

Examples:
  membox schedule                  # daily at 02:00
  membox schedule --interval hourly
  membox schedule --bwlimit 5000   # throttle to ~5 MB/s
  membox schedule --remove         # unload and delete the job`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts := scheduleOpts{
				interval: interval,
				bwlimit:  bwlimit,
				nice:     nice,
				remove:   remove,
			}
			if runtime.GOOS == "linux" {
				return scheduleLinux(a, opts)
			}
			return scheduleDarwin(a, opts)
		},
	}

	cmd.Flags().StringVar(&interval, "interval", "daily", "schedule interval: hourly, daily, weekly")
	cmd.Flags().IntVar(&bwlimit, "bwlimit", 0, "rsync bandwidth limit in KB/s (0 = unlimited)")
	cmd.Flags().BoolVar(&nice, "nice", true, "run at low CPU/IO priority")
	cmd.Flags().BoolVar(&remove, "remove", false, "remove the scheduled job")

	return cmd
}

type scheduleOpts struct {
	interval string
	bwlimit  int
	nice     bool
	remove   bool
}

// ── macOS ─────────────────────────────────────────────────────────────────────

const darwinLabel = "com.memorybox.backup"

func scheduleDarwin(a *app.App, opts scheduleOpts) error {
	plistDir := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents")
	plistPath := filepath.Join(plistDir, darwinLabel+".plist")

	if opts.remove {
		exec.Command("launchctl", "unload", plistPath).Run() //nolint:errcheck
		if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove plist: %w", err)
		}
		a.Printer.Success("Scheduled backup removed")
		return nil
	}

	if err := os.MkdirAll(plistDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", plistDir, err)
	}

	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	plistContent := darwinPlist(selfPath, opts)
	if err := os.WriteFile(plistPath, []byte(plistContent), 0o644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}

	// Unload first in case it was already loaded, then load fresh.
	exec.Command("launchctl", "unload", plistPath).Run() //nolint:errcheck
	if out, err := exec.Command("launchctl", "load", "-w", plistPath).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl load: %w\n%s", err, out)
	}

	a.Printer.Success(fmt.Sprintf("Scheduled backup installed: %s", plistPath))
	a.Printer.Info(fmt.Sprintf("  launchctl list %s", darwinLabel))
	return nil
}

func darwinPlist(selfPath string, opts scheduleOpts) string {
	var calendarInterval string
	switch opts.interval {
	case "hourly":
		calendarInterval = "        <dict><key>Minute</key><integer>0</integer></dict>"
	case "weekly":
		calendarInterval = "        <dict><key>Weekday</key><integer>0</integer><key>Hour</key><integer>2</integer><key>Minute</key><integer>0</integer></dict>"
	default: // daily
		calendarInterval = "        <dict><key>Hour</key><integer>2</integer><key>Minute</key><integer>0</integer></dict>"
	}

	args := []string{selfPath, "backup"}
	if opts.bwlimit > 0 {
		args = append(args, fmt.Sprintf("--bwlimit=%d", opts.bwlimit))
	}

	var argStrings strings.Builder
	for _, a := range args {
		argStrings.WriteString(fmt.Sprintf("        <string>%s</string>\n", a))
	}

	nice := ""
	if opts.nice {
		nice = "    <key>Nice</key>\n    <integer>10</integer>\n"
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
%s    </array>
    <key>StartCalendarInterval</key>
    <array>
%s
    </array>
    <key>StandardOutPath</key>
    <string>%s/Library/Logs/memorybox.log</string>
    <key>StandardErrorPath</key>
    <string>%s/Library/Logs/memorybox.log</string>
    <key>RunAtLoad</key>
    <false/>
%s</dict>
</plist>
`, darwinLabel, argStrings.String(), calendarInterval,
		os.Getenv("HOME"), os.Getenv("HOME"), nice)
}

// ── Linux ─────────────────────────────────────────────────────────────────────

func scheduleLinux(a *app.App, opts scheduleOpts) error {
	unitDir := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user")
	servicePath := filepath.Join(unitDir, "membox-backup.service")
	timerPath := filepath.Join(unitDir, "membox-backup.timer")

	if opts.remove {
		exec.Command("systemctl", "--user", "disable", "--now", "membox-backup.timer").Run() //nolint:errcheck
		os.Remove(servicePath)                                                                //nolint:errcheck
		os.Remove(timerPath)                                                                  //nolint:errcheck
		exec.Command("systemctl", "--user", "daemon-reload").Run()                           //nolint:errcheck
		a.Printer.Success("Scheduled backup removed")
		return nil
	}

	if err := os.MkdirAll(unitDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", unitDir, err)
	}

	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	service := linuxService(selfPath, opts)
	if err := os.WriteFile(servicePath, []byte(service), 0o644); err != nil {
		return fmt.Errorf("write service: %w", err)
	}

	timer := linuxTimer(opts)
	if err := os.WriteFile(timerPath, []byte(timer), 0o644); err != nil {
		return fmt.Errorf("write timer: %w", err)
	}

	exec.Command("systemctl", "--user", "daemon-reload").Run() //nolint:errcheck
	if out, err := exec.Command("systemctl", "--user", "enable", "--now", "membox-backup.timer").CombinedOutput(); err != nil {
		return fmt.Errorf("enable timer: %w\n%s", err, out)
	}

	a.Printer.Success(fmt.Sprintf("Scheduled backup installed: %s", timerPath))
	a.Printer.Info("  systemctl --user status membox-backup.timer")
	return nil
}

func linuxService(selfPath string, opts scheduleOpts) string {
	cmd := selfPath + " backup"
	if opts.bwlimit > 0 {
		cmd += fmt.Sprintf(" --bwlimit=%d", opts.bwlimit)
	}
	nice := ""
	if opts.nice {
		nice = "Nice=10\nIOSchedulingClass=idle\n"
	}
	return fmt.Sprintf(`[Unit]
Description=Memory Box backup
After=network.target

[Service]
Type=oneshot
ExecStart=%s
%s
[Install]
WantedBy=default.target
`, cmd, nice)
}

func linuxTimer(opts scheduleOpts) string {
	var onCalendar string
	switch opts.interval {
	case "hourly":
		onCalendar = "hourly"
	case "weekly":
		onCalendar = "weekly"
	default:
		onCalendar = "*-*-* 02:00:00"
	}
	return fmt.Sprintf(`[Unit]
Description=Memory Box backup timer

[Timer]
OnCalendar=%s
Persistent=true

[Install]
WantedBy=timers.target
`, onCalendar)
}
