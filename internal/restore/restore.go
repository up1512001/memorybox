// Package restore finds and recovers files from backup archives.
package restore

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/up1512001/memorybox/internal/snapshot"
)

// Match is one search result: a file found in a specific snapshot.
type Match struct {
	SnapshotKey  string
	Path         string // relative to backup root
	AbsolutePath string // full path on external drive
	Size         int64
}

// FindOpts controls how the search is performed.
type FindOpts struct {
	Pattern     string // filepath.Match pattern or substring
	SnapshotKey string // if non-empty, search only this snapshot
}

// CopyOpts controls how a matched file is restored.
type CopyOpts struct {
	Destination string // target directory on the user's Mac
	DryRun      bool
}

// Scanner searches all archive directories for files matching a pattern.
// It streams results to out and closes it on return.
type Scanner struct {
	store      *snapshot.Store
	backupDir  string
	archiveDir string
}

// New returns a Scanner.
func New(store *snapshot.Store, backupDir, archiveDir string) *Scanner {
	return &Scanner{store: store, backupDir: backupDir, archiveDir: archiveDir}
}

// Find searches for files matching opts.Pattern across archives (and current backup).
func (sc *Scanner) Find(ctx context.Context, opts FindOpts, out chan<- Match) error {
	defer close(out)

	snaps, err := sc.store.List(ctx)
	if err != nil {
		return err
	}

	// Filter to specific snapshot if requested.
	if opts.SnapshotKey != "" {
		var filtered []snapshot.Snapshot
		for _, s := range snaps {
			if s.Key == opts.SnapshotKey || strings.HasPrefix(s.Key, opts.SnapshotKey) {
				filtered = append(filtered, s)
				break
			}
		}
		snaps = filtered
	}

	// Search each snapshot's archive directory.
	for _, snap := range snaps {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := sc.searchDir(ctx, snap.Key, snap.ArchivePath, opts.Pattern, out); err != nil {
			return err
		}
	}

	// Also search the current live backup.
	return sc.searchDir(ctx, "latest", sc.backupDir, opts.Pattern, out)
}

func (sc *Scanner) searchDir(ctx context.Context, key, dir, pattern string, out chan<- Match) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || ctx.Err() != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		if !matchPattern(pattern, rel) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		out <- Match{
			SnapshotKey:  key,
			Path:         rel,
			AbsolutePath: path,
			Size:         info.Size(),
		}
		return nil
	})
}

// Copy restores a matched file to opts.Destination.
func (sc *Scanner) Copy(match Match, opts CopyOpts) error {
	dest := filepath.Join(opts.Destination, filepath.Base(match.Path))
	if opts.DryRun {
		fmt.Printf("would copy: %s → %s\n", match.AbsolutePath, dest)
		return nil
	}
	if err := os.MkdirAll(opts.Destination, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", opts.Destination, err)
	}
	return copyFile(match.AbsolutePath, dest)
}

// matchPattern returns true if name contains pattern as a substring,
// or if filepath.Match(pattern, name) succeeds.
func matchPattern(pattern, name string) bool {
	if strings.Contains(name, pattern) {
		return true
	}
	matched, _ := filepath.Match(pattern, filepath.Base(name))
	return matched
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
