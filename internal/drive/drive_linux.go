//go:build linux

// Package drive probes the external backup drive for health and space.
package drive

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// Info holds drive health metrics.
type Info struct {
	MountPath  string
	TotalBytes int64
	FreeBytes  int64
	UsedBytes  int64
	FSType     string
}

// Prober checks backup drive health.
type Prober struct{}

// New returns a Prober.
func New() *Prober { return &Prober{} }

// Probe returns drive info for the given mount path.
func (p *Prober) Probe(_ context.Context, mountPath string) (Info, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(mountPath, &stat); err != nil {
		return Info{}, fmt.Errorf("statfs %s: %w", mountPath, err)
	}
	total := int64(stat.Blocks) * stat.Bsize
	free := int64(stat.Bavail) * stat.Bsize
	return Info{
		MountPath:  mountPath,
		TotalBytes: total,
		FreeBytes:  free,
		UsedBytes:  total - free,
		FSType:     fsTypeName(mountPath),
	}, nil
}

// HasSpace reports whether at least required bytes are free on mountPath.
func (p *Prober) HasSpace(_ context.Context, mountPath string, required int64) (bool, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(mountPath, &stat); err != nil {
		return false, fmt.Errorf("statfs %s: %w", mountPath, err)
	}
	free := int64(stat.Bavail) * stat.Bsize
	return free >= required, nil
}

// EstimateSourceSize returns a rough byte count for the given path using du.
func EstimateSourceSize(path string) (int64, error) {
	out, err := exec.Command("du", "-sk", path).Output()
	if err != nil {
		return 0, nil
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return 0, nil
	}
	kb, _ := strconv.ParseInt(fields[0], 10, 64)
	return kb * 1024, nil
}

func fsTypeName(mountPath string) string {
	out, err := exec.Command("df", "-T", mountPath).Output()
	if err != nil {
		return "unknown"
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return "unknown"
	}
	fields := strings.Fields(lines[1])
	if len(fields) >= 2 {
		return fields[1]
	}
	return "unknown"
}
