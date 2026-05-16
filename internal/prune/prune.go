// Package prune implements snapshot retention policies.
package prune

import (
	"context"
	"fmt"
	"time"

	"github.com/up1512001/memorybox/internal/snapshot"
)

// Opts controls which snapshots are deleted.
type Opts struct {
	OlderThanDays int  // delete snapshots older than N days (0 = no age limit)
	KeepLast      int  // always keep the most recent N snapshots (0 = no keep-N)
	DryRun        bool // report without deleting
	Force         bool // skip confirmation (handled at CLI layer)
}

// Result describes what was (or would be) pruned.
type Result struct {
	Removed []snapshot.Snapshot
	Freed   int64 // approximate bytes (0 when DryRun)
}

// Pruner evaluates retention policy against the snapshot store.
type Pruner struct {
	store *snapshot.Store
}

// New returns a Pruner backed by the given store.
func New(store *snapshot.Store) *Pruner {
	return &Pruner{store: store}
}

// Candidates returns the snapshots that would be deleted under opts.
func (p *Pruner) Candidates(ctx context.Context, opts Opts) ([]snapshot.Snapshot, error) {
	all, err := p.store.List(ctx) // newest-first
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -opts.OlderThanDays)
	protected := opts.KeepLast

	var candidates []snapshot.Snapshot
	for i, snap := range all {
		keepByIndex := protected > 0 && i < protected
		tooRecent := opts.OlderThanDays > 0 && snap.CreatedAt.After(cutoff)

		if keepByIndex || tooRecent {
			continue
		}
		candidates = append(candidates, snap)
	}
	return candidates, nil
}

// Prune deletes candidates according to opts and returns what happened.
func (p *Pruner) Prune(ctx context.Context, opts Opts) (Result, error) {
	candidates, err := p.Candidates(ctx, opts)
	if err != nil {
		return Result{}, err
	}

	var res Result
	for _, snap := range candidates {
		if opts.DryRun {
			res.Removed = append(res.Removed, snap)
			continue
		}
		if err := p.store.Delete(ctx, snap.Key); err != nil {
			return res, fmt.Errorf("prune %s: %w", snap.Key, err)
		}
		res.Removed = append(res.Removed, snap)
	}
	return res, nil
}
