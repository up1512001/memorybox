package rclone

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/up1512001/memorybox/internal/rsync"
)

// Runner implements rsync.Runner using the rclone binary as transport.
// It accepts rclone-native argv built by BuildSyncArgs.
type Runner struct {
	// BinaryPath is the path to the rclone binary. Defaults to "rclone" ($PATH).
	BinaryPath string
}

func (r *Runner) binary() string {
	if r.BinaryPath != "" {
		return r.BinaryPath
	}
	return "rclone"
}

// Run executes rclone with the given args, sends each --combined output line to
// lines, and returns aggregate stats. Implements rsync.Runner.
func (r *Runner) Run(ctx context.Context, args []string, lines chan<- rsync.Line) (*rsync.Stats, error) {
	defer close(lines)

	cmd := exec.CommandContext(ctx, r.binary(), args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("rclone stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("rclone stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("rclone start: %w", err)
	}

	outBytes, _ := io.ReadAll(stdout)
	errBytes, _ := io.ReadAll(stderr)

	if err := cmd.Wait(); err != nil {
		lines <- rsync.Line{Raw: string(errBytes), IsError: true}
		return nil, fmt.Errorf("rclone: %w", err)
	}

	// Stream --combined lines to caller.
	for _, raw := range splitLines(string(outBytes)) {
		if raw == "" {
			continue
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case lines <- rsync.Line{Raw: raw}:
		}
	}

	// Parse stats from stderr (rclone writes stats to stderr).
	stats := ParseStats(string(errBytes))
	return stats, nil
}

// Exec runs rclone with args, waiting for completion and returning combined output.
// Used for push/pull/list operations that don't need streaming.
func Exec(ctx context.Context, binaryPath string, args ...string) ([]byte, error) {
	if binaryPath == "" {
		binaryPath = "rclone"
	}
	out, err := exec.CommandContext(ctx, binaryPath, args...).CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("rclone %s: %w\n%s", strings.Join(args[:min(2, len(args))], " "), err, out)
	}
	return out, nil
}

// CheckInstalled returns an error if rclone is not found in PATH.
func CheckInstalled(binaryPath string) error {
	if binaryPath == "" {
		binaryPath = "rclone"
	}
	if _, err := exec.LookPath(binaryPath); err != nil {
		return fmt.Errorf("rclone not found — install from https://rclone.org/install/")
	}
	return nil
}

// CheckRemote verifies that a rclone remote path is accessible.
func CheckRemote(ctx context.Context, binaryPath, remotePath string) error {
	if binaryPath == "" {
		binaryPath = "rclone"
	}
	out, err := exec.CommandContext(ctx, binaryPath, "lsd", remotePath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("cannot access %q: %w\n%s\nRun `rclone config` to set up the remote", remotePath, err, out)
	}
	return nil
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
