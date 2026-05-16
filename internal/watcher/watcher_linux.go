//go:build linux

// Package watcher detects when the backup drive is connected and triggers backup.
package watcher

import (
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

// WatchDirs returns the directories to monitor for new drive mounts on Linux.
func WatchDirs() []string {
	user := os.Getenv("USER")
	return []string{
		"/media/" + user,
		"/run/media/" + user,
		"/mnt",
	}
}

// IsMountEvent returns true when the fsnotify event indicates a new mount appeared.
func IsMountEvent(event fsnotify.Event) bool {
	return event.Has(fsnotify.Create) && isDir(event.Name)
}

// MountPath extracts the mount path from a create event.
func MountPath(event fsnotify.Event) string {
	return event.Name
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir() && filepath.Dir(path) != path
}
