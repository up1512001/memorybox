// Package snapshot manages the lifecycle of backup snapshots on the drive.
package snapshot

import "time"

// Snapshot is one point-in-time backup.
// The Key is an RFC3339 UTC timestamp — lexicographic order == chronological order,
// so no parsing is needed for sort/compare operations.
type Snapshot struct {
	Key          string    // e.g. "2026-05-16T14:30:00Z"
	Label        string    // user-supplied --message, may be empty
	CreatedAt    time.Time
	Sections     []string  // section names that were included
	ManifestPath string    // absolute path to the .manifest file
	ArchivePath  string    // absolute path to the rsync --backup-dir
}

// SnapshotMeta holds caller-supplied data needed to create a snapshot.
type SnapshotMeta struct {
	Label    string
	Sections []string
}
