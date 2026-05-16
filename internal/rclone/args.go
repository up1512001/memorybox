// Package rclone wraps the rclone subprocess as a drop-in transport for membox.
package rclone

import "fmt"

// SyncOpts holds parameters for a single rclone sync operation.
type SyncOpts struct {
	Source      string
	Destination string // rclone remote path, e.g. "r2:bucket/membox/photos"
	Excludes    []string
	DryRun      bool
	BwLimit     int // KB/s; 0 = unlimited
}

// CopyOpts holds parameters for a single rclone file copy (used for restore).
type CopyOpts struct {
	Source      string // full rclone path, e.g. "r2:bucket/membox/photos/img.jpg"
	Destination string // local destination directory
}

// BuildSyncArgs constructs the rclone argv for a section sync.
// Uses --combined - (stdout) for itemised change output and --stats for summary.
func BuildSyncArgs(opts SyncOpts) []string {
	args := []string{
		"sync",
		"--combined", "-", // itemised changes to stdout: + new, * changed, - deleted, = same
		"--stats", "0",    // disable periodic stats; we parse the final summary via --stats-one-line-date
		"--stats-one-line",
		"--stats-unit=bytes",
	}

	if opts.DryRun {
		args = append(args, "--dry-run")
	}
	if opts.BwLimit > 0 {
		args = append(args, "--bwlimit", kiloString(opts.BwLimit))
	}
	for _, ex := range opts.Excludes {
		args = append(args, "--exclude", ex)
	}

	args = append(args, opts.Source, opts.Destination)
	return args
}

// BuildCopyArgs constructs rclone argv to copy a single file to a local directory.
func BuildCopyArgs(opts CopyOpts) []string {
	return []string{"copy", opts.Source, opts.Destination}
}

// BuildPushManifestArgs returns argv to push a local manifest file to the cloud.
func BuildPushManifestArgs(localPath, remotePath string) []string {
	return []string{"copyto", localPath, remotePath}
}

// BuildPullManifestArgs returns argv to pull a remote manifest to a local path.
func BuildPullManifestArgs(remotePath, localPath string) []string {
	return []string{"copyto", remotePath, localPath}
}

// BuildListArgs returns argv to list files at a remote path as JSON.
func BuildListArgs(remotePath string) []string {
	return []string{"lsjson", "--files-only", "--no-mimetype", remotePath}
}

func kiloString(kb int) string {
	if kb >= 1024 {
		return fmt.Sprintf("%dM", kb/1024)
	}
	return fmt.Sprintf("%dk", kb)
}

// RemoteManifestPath returns the remote path for a manifest file given the
// rclone base path and snapshot key.
func RemoteManifestPath(rclonePath, snapshotKey string) string {
	return rclonePath + "/manifests/" + snapshotKey + ".manifest"
}
