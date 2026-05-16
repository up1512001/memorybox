package cmd

import (
	"os"
	"os/exec"
)

// runToFile executes a command and writes stdout to path.
// Errors are silently ignored (optional inventory commands).
func runToFile(name string, args []string, path string) {
	out, err := exec.Command(name, args...).Output()
	if err != nil || len(out) == 0 {
		return
	}
	os.WriteFile(path, out, 0o644)
}

// runToFileWithShell runs a shell command string and writes stdout to path.
func runToFileWithShell(script, path string) {
	out, err := exec.Command("bash", "-c", script).Output()
	if err != nil || len(out) == 0 {
		return
	}
	os.WriteFile(path, out, 0o644)
}
