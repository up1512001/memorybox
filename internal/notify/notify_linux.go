//go:build linux

// Package notify sends OS-level backup notifications.
package notify

import (
	"os/exec"
)

// Success sends a success notification via notify-send (no-op if not installed).
func Success(title, message string) {
	if path, err := exec.LookPath("notify-send"); err == nil {
		exec.Command(path, "-i", "dialog-information", title, message).Run() //nolint:errcheck
	}
}

// Failure sends a failure notification via notify-send (no-op if not installed).
func Failure(title, message string) {
	if path, err := exec.LookPath("notify-send"); err == nil {
		exec.Command(path, "-i", "dialog-error", "-u", "critical", title, message).Run() //nolint:errcheck
	}
}
