//go:build darwin

package config

import (
	"os"
	"path/filepath"
	"strings"
)

// AvailableVolumes returns mount points visible under /Volumes, skipping hidden entries.
func AvailableVolumes() []string {
	entries, err := os.ReadDir("/Volumes")
	if err != nil {
		return nil
	}
	var vols []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		vols = append(vols, filepath.Join("/Volumes", e.Name()))
	}
	return vols
}
