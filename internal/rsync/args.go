package rsync

import "path/filepath"

// SectionRunOpts holds all parameters for a single rsync section.
type SectionRunOpts struct {
	Source      string
	Destination string
	ArchiveDir  string   // --backup-dir destination
	Excludes    []string
	Delete      bool
	Verbose     bool
	DryRun      bool
}

// BuildArgs constructs the rsync argv for the given options.
// No shell is involved — paths with spaces are passed as raw strings.
func BuildArgs(opts SectionRunOpts) []string {
	args := []string{
		"-a",                   // archive: preserves permissions, symlinks, etc.
		"--stats",              // emit transfer stats
		"--itemize-changes",    // emit per-file change codes
		"--backup",             // save overwritten/deleted files
		"--backup-dir=" + opts.ArchiveDir,
	}

	if opts.Delete {
		args = append(args, "--delete")
	}
	if opts.DryRun {
		args = append(args, "--dry-run")
	}
	if opts.Verbose {
		args = append(args, "--progress")
	}

	for _, ex := range opts.Excludes {
		args = append(args, "--exclude="+ex)
	}

	// Ensure source ends with / so rsync syncs directory contents, not the dir itself.
	src := opts.Source
	if src != "" && src[len(src)-1] != '/' {
		src += "/"
	}

	args = append(args, src)
	args = append(args, filepath.Clean(opts.Destination)+"/")
	return args
}
