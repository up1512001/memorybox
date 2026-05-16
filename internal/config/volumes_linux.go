//go:build linux

package config

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// AvailableVolumes returns mount points that look like external drives on Linux.
// Scans /media/$USER, /run/media/$USER (Fedora/openSUSE), and /mnt.
func AvailableVolumes() []string {
	user := os.Getenv("USER")
	bases := []string{
		"/media/" + user,
		"/run/media/" + user,
		"/mnt",
	}

	var vols []string
	for _, base := range bases {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".") {
				continue
			}
			path := filepath.Join(base, e.Name())
			if isMountPoint(path) {
				vols = append(vols, path)
			}
		}
	}
	return vols
}

// isMountPoint returns true when path's device differs from its parent's device.
func isMountPoint(path string) bool {
	var pathStat, parentStat syscall.Stat_t
	if err := syscall.Lstat(path, &pathStat); err != nil {
		return false
	}
	if err := syscall.Lstat(filepath.Dir(path), &parentStat); err != nil {
		return false
	}
	return pathStat.Dev != parentStat.Dev
}
