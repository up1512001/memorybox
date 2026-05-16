//go:build darwin

// Package notify sends OS-level backup notifications.
package notify

import (
	"fmt"
	"os/exec"
)

// Success sends a success notification via macOS Notification Center.
func Success(title, message string) {
	script := fmt.Sprintf(
		`display notification %q with title %q sound name "Glass"`,
		message, title,
	)
	exec.Command("osascript", "-e", script).Run() //nolint:errcheck
}

// Failure sends a failure notification via macOS Notification Center.
func Failure(title, message string) {
	script := fmt.Sprintf(
		`display notification %q with title %q sound name "Basso"`,
		message, title,
	)
	exec.Command("osascript", "-e", script).Run() //nolint:errcheck
}
