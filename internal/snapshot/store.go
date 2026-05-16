package snapshot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/up1512001/memorybox/internal/manifest"
)

// ErrNoSnapshots is returned when no snapshots exist.
var ErrNoSnapshots = errors.New("no snapshots found")

// ErrNotFound is returned when a specific snapshot key does not exist.
var ErrNotFound = errors.New("snapshot not found")

// Store is the filesystem-backed snapshot store.
type Store struct {
	manifestDir string
	archiveDir  string
}

// NewStore returns a Store rooted at the given directories.
func NewStore(manifestDir, archiveDir string) *Store {
	return &Store{manifestDir: manifestDir, archiveDir: archiveDir}
}

// List returns all snapshots newest-first.
func (s *Store) List(_ context.Context) ([]Snapshot, error) {
	entries, err := os.ReadDir(s.manifestDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list snapshots: %w", err)
	}

	var snaps []Snapshot
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".manifest") {
			continue
		}
		key := strings.TrimSuffix(e.Name(), ".manifest")
		snap, err := s.buildSnapshot(key)
		if err != nil {
			continue
		}
		snaps = append(snaps, snap)
	}

	// Sort newest-first (ISO8601 keys are lexicographically ordered).
	sort.Slice(snaps, func(i, j int) bool {
		return snaps[i].Key > snaps[j].Key
	})
	return snaps, nil
}

// Get returns the snapshot for an exact key.
func (s *Store) Get(_ context.Context, key string) (Snapshot, error) {
	path := filepath.Join(s.manifestDir, key+".manifest")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return Snapshot{}, fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	return s.buildSnapshot(key)
}

// PreviousOf returns the snapshot immediately before the given key.
func (s *Store) PreviousOf(ctx context.Context, key string) (Snapshot, error) {
	all, err := s.List(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	// all is newest-first; find key index.
	for i, snap := range all {
		if snap.Key == key {
			if i+1 >= len(all) {
				return Snapshot{}, fmt.Errorf("%w: %s is the oldest snapshot", ErrNotFound, key)
			}
			return all[i+1], nil
		}
	}
	return Snapshot{}, fmt.Errorf("%w: %s", ErrNotFound, key)
}

// Latest returns the most recent snapshot.
func (s *Store) Latest(ctx context.Context) (Snapshot, error) {
	all, err := s.List(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	if len(all) == 0 {
		return Snapshot{}, ErrNoSnapshots
	}
	return all[0], nil
}

// FindByPrefix returns the latest snapshot whose key starts with prefix.
// Useful for partial date matches like "2026-05-16".
func (s *Store) FindByPrefix(ctx context.Context, prefix string) (Snapshot, error) {
	all, err := s.List(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	for _, snap := range all {
		if strings.HasPrefix(snap.Key, prefix) {
			return snap, nil
		}
	}
	return Snapshot{}, fmt.Errorf("%w: no snapshot with prefix %q", ErrNotFound, prefix)
}

// Create writes a new snapshot record and returns it.
func (s *Store) Create(_ context.Context, meta SnapshotMeta) (Snapshot, error) {
	key := time.Now().UTC().Format(time.RFC3339)
	key = strings.ReplaceAll(key, ":", "-") // filesystem-safe

	if err := os.MkdirAll(s.manifestDir, 0o755); err != nil {
		return Snapshot{}, fmt.Errorf("create snapshot dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(s.archiveDir, key), 0o755); err != nil {
		return Snapshot{}, fmt.Errorf("create archive dir: %w", err)
	}

	snap, _ := s.buildSnapshot(key)
	snap.Label = meta.Label
	snap.Sections = meta.Sections
	return snap, nil
}

// Delete removes a snapshot's manifest and archive directory.
func (s *Store) Delete(_ context.Context, key string) error {
	manifestPath := filepath.Join(s.manifestDir, key+".manifest")
	archivePath := filepath.Join(s.archiveDir, key)

	if err := os.Remove(manifestPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete manifest: %w", err)
	}
	if err := os.RemoveAll(archivePath); err != nil {
		return fmt.Errorf("delete archive: %w", err)
	}
	return nil
}

// ManifestWriter returns a writer for the snapshot's manifest file.
func (s *Store) ManifestWriter(key string) (*manifest.Writer, func() error, error) {
	path := filepath.Join(s.manifestDir, key+".manifest")
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("create manifest: %w", err)
	}
	w := manifest.NewWriter(f)
	close := func() error {
		if err := w.Flush(); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	}
	return w, close, nil
}

// ManifestReader returns a streaming reader for the snapshot's manifest.
func (s *Store) ManifestReader(key string) (*manifest.Reader, func() error, error) {
	path := filepath.Join(s.manifestDir, key+".manifest")
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open manifest %s: %w", key, err)
	}
	r, err := manifest.NewReader(f)
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	return r, f.Close, nil
}

func (s *Store) buildSnapshot(key string) (Snapshot, error) {
	t, _ := time.Parse("2006-01-02T15-04-05Z", key)
	return Snapshot{
		Key:          key,
		CreatedAt:    t,
		ManifestPath: filepath.Join(s.manifestDir, key+".manifest"),
		ArchivePath:  filepath.Join(s.archiveDir, key),
	}, nil
}
