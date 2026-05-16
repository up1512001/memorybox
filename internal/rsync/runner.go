// Package rsync wraps the rsync subprocess.
package rsync

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// Stats holds the aggregate numbers parsed from rsync --stats output.
type Stats struct {
	FilesTransferred int64
	TotalFileSize    int64
	TotalTransferred int64
}

// Runner executes rsync and streams itemize-change lines to the caller.
type Runner interface {
	// Run executes rsync with args, sending each output line to lines.
	// It closes lines when rsync exits (or ctx is cancelled).
	Run(ctx context.Context, args []string, lines chan<- Line) (*Stats, error)
}

// Line is one line emitted by rsync --itemize-changes.
type Line struct {
	Raw     string
	IsError bool
}

// Exec is the real Runner that shells out to rsync.
type Exec struct {
	// Path to the rsync binary. Defaults to "rsync" (uses $PATH).
	BinaryPath string
}

func (e *Exec) binary() string {
	if e.BinaryPath != "" {
		return e.BinaryPath
	}
	return "rsync"
}

func (e *Exec) Run(ctx context.Context, args []string, lines chan<- Line) (*Stats, error) {
	defer close(lines)

	cmd := exec.CommandContext(ctx, e.binary(), args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("rsync stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("rsync stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("rsync start: %w", err)
	}

	// Collect all output into a buffer so we can parse stats and stream lines.
	outBytes, _ := io.ReadAll(stdout)
	errBytes, _ := io.ReadAll(stderr)

	if err := cmd.Wait(); err != nil {
		// rsync exit 24 (vanished files) is non-fatal for backups.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 24 {
			err = nil
		}
		if err != nil {
			// Send error output as lines.
			lines <- Line{Raw: string(errBytes), IsError: true}
			return nil, fmt.Errorf("rsync: %w", err)
		}
	}

	// Stream output lines to caller.
	for _, raw := range splitLines(string(outBytes)) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case lines <- Line{Raw: raw}:
		}
	}

	stats := parseStats(string(outBytes))
	return stats, nil
}
