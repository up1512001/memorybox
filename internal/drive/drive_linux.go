//go:build linux

// Package drive probes the external backup drive for health and space.
// Linux stub — Memory Box is Mac-only, but this allows `go build` on Linux
// (e.g. GitHub Codespaces) for testing non-drive-dependent code paths.
package drive

import (
	"context"
	"fmt"
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

func (p *Prober) Probe(_ context.Context, mountPath string) (Info, error) {
	return Info{}, fmt.Errorf("drive probing not supported on Linux (Memory Box is Mac-only)")
}

func (p *Prober) HasSpace(_ context.Context, _ string, _ int64) (bool, error) {
	return true, nil // non-fatal on Linux
}

// EstimateSourceSize is a no-op on Linux.
func EstimateSourceSize(_ string) (int64, error) { return 0, nil }
