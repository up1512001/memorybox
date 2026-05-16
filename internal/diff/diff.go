// Package diff compares two snapshots using O(n) sort-merge.
package diff

import (
	"context"
	"fmt"

	"github.com/up1512001/memorybox/internal/manifest"
	"github.com/up1512001/memorybox/internal/snapshot"
)

// ChangeType classifies a file change between two snapshots.
type ChangeType int

const (
	Added    ChangeType = iota // in new snapshot only
	Deleted                    // in old snapshot only
	Modified                   // in both, but size or mtime changed
)

func (c ChangeType) String() string {
	switch c {
	case Added:
		return "A"
	case Deleted:
		return "D"
	case Modified:
		return "M"
	}
	return "?"
}

// Entry is one file-level change between two snapshots.
type Entry struct {
	Change ChangeType
	Path   string
	From   manifest.Entry // zero value when Added
	To     manifest.Entry // zero value when Deleted
}

// Stat holds aggregate change counts.
type Stat struct {
	Added    int
	Deleted  int
	Modified int
}

// Total returns the total number of changed files.
func (s Stat) Total() int { return s.Added + s.Deleted + s.Modified }

// Differ compares two snapshots via streaming sort-merge.
type Differ struct {
	store *snapshot.Store
}

// New returns a Differ backed by the given store.
func New(store *snapshot.Store) *Differ {
	return &Differ{store: store}
}

// Diff streams DiffEntry values for all changed files between fromKey and toKey.
// If toKey is empty, the latest snapshot is used.
// out is closed when Diff returns.
func (d *Differ) Diff(ctx context.Context, fromKey, toKey string, out chan<- Entry) error {
	defer close(out)

	leftReader, closeLeft, err := d.store.ManifestReader(fromKey)
	if err != nil {
		return fmt.Errorf("diff: open from-manifest %s: %w", fromKey, err)
	}
	defer closeLeft()

	rightReader, closeRight, err := d.store.ManifestReader(toKey)
	if err != nil {
		return fmt.Errorf("diff: open to-manifest %s: %w", toKey, err)
	}
	defer closeRight()

	events := make(chan manifest.MergeEvent, 512)
	errCh := make(chan error, 1)

	go func() {
		errCh <- manifest.Merge(ctx, leftReader, rightReader, events)
	}()

	for ev := range events {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		de, changed := toEntry(ev)
		if !changed {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case out <- de:
		}
	}

	return <-errCh
}

// Stat returns only aggregate counts without streaming file paths.
func (d *Differ) Stat(ctx context.Context, fromKey, toKey string) (Stat, error) {
	out := make(chan Entry, 512)
	var stat Stat

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Diff(ctx, fromKey, toKey, out)
	}()

	for e := range out {
		switch e.Change {
		case Added:
			stat.Added++
		case Deleted:
			stat.Deleted++
		case Modified:
			stat.Modified++
		}
	}
	return stat, <-errCh
}

func toEntry(ev manifest.MergeEvent) (Entry, bool) {
	switch {
	case ev.Left == nil && ev.Right != nil:
		return Entry{Change: Added, Path: ev.Right.Path, To: *ev.Right}, true
	case ev.Left != nil && ev.Right == nil:
		return Entry{Change: Deleted, Path: ev.Left.Path, From: *ev.Left}, true
	case ev.Left != nil && ev.Right != nil:
		if ev.Left.Size != ev.Right.Size || ev.Left.MTime != ev.Right.MTime {
			return Entry{Change: Modified, Path: ev.Left.Path, From: *ev.Left, To: *ev.Right}, true
		}
	}
	return Entry{}, false
}
