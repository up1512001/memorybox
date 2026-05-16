//go:build darwin

// Package watcher detects when the backup drive is connected and triggers backup.
package watcher

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// WatchDirs returns the directories to monitor for new drive mounts on macOS.
func WatchDirs() []string {
	return []string{"/Volumes"}
}

// IsMountEvent returns true when the fsnotify event indicates a new volume was mounted.
func IsMountEvent(event fsnotify.Event) bool {
	return event.Has(fsnotify.Create) && isVolumeDir(event.Name)
}

// MountPath extracts the mount path from a create event.
func MountPath(event fsnotify.Event) string {
	return event.Name
}

func isVolumeDir(path string) bool {
	if !strings.HasPrefix(path, "/Volumes/") {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir() && filepath.Dir(path) == "/Volumes"
}
