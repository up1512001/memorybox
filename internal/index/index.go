// Package index maintains a BoltDB-backed file index for instant restore lookups.
// Schema: one bucket per snapshot key, key=relative path, value="size\tmtime_unix".
package index

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

const dbFile = "index.db"

// Entry is one search result from the index.
type Entry struct {
	Snapshot string
	Path     string
	Size     int64
	MTime    time.Time
}

// Index wraps a BoltDB database.
type Index struct {
	db *bolt.DB
}

// Open opens (or creates) the index at dir/index.db.
func Open(dir string) (*Index, error) {
	db, err := bolt.Open(filepath.Join(dir, dbFile), 0o600, &bolt.Options{Timeout: 2_000_000_000})
	if err != nil {
		return nil, fmt.Errorf("open index: %w", err)
	}
	return &Index{db: db}, nil
}

// Close closes the underlying database.
func (idx *Index) Close() error { return idx.db.Close() }

// IndexFromReader reads (path, size, mtime) tuples from next until io.EOF,
// storing all under snapshotKey. Replaces any previous data for that key.
func (idx *Index) IndexFromReader(snapshotKey string, next func() (path string, size int64, mtime time.Time, err error)) error {
	return idx.db.Update(func(tx *bolt.Tx) error {
		tx.DeleteBucket([]byte(snapshotKey)) //nolint:errcheck
		b, err := tx.CreateBucket([]byte(snapshotKey))
		if err != nil {
			return err
		}
		for {
			path, size, mtime, err := next()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			val := strconv.FormatInt(size, 10) + "\t" + strconv.FormatInt(mtime.Unix(), 10)
			if putErr := b.Put([]byte(path), []byte(val)); putErr != nil {
				return putErr
			}
		}
	})
}

// Search streams entries whose path contains pattern (or matches as a glob)
// across all snapshots, sending results to out. Closes out on return.
func (idx *Index) Search(pattern string, out chan<- Entry) error {
	defer close(out)
	return idx.db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(name []byte, b *bolt.Bucket) error {
			snap := string(name)
			return b.ForEach(func(k, v []byte) error {
				path := string(k)
				if !matchPath(pattern, path) {
					return nil
				}
				size, mtime := decode(string(v))
				out <- Entry{Snapshot: snap, Path: path, Size: size, MTime: mtime}
				return nil
			})
		})
	})
}

// DeleteSnapshot removes a snapshot's entries from the index.
func (idx *Index) DeleteSnapshot(snapshotKey string) error {
	return idx.db.Update(func(tx *bolt.Tx) error {
		err := tx.DeleteBucket([]byte(snapshotKey))
		if err == bolt.ErrBucketNotFound {
			return nil
		}
		return err
	})
}

// Stats returns the count of indexed snapshots and total file entries.
func (idx *Index) Stats() (snapshots, entries int, err error) {
	err = idx.db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(_ []byte, b *bolt.Bucket) error {
			snapshots++
			entries += b.Stats().KeyN
			return nil
		})
	})
	return
}

// ── helpers ───────────────────────────────────────────────────────────────────

func decode(v string) (size int64, mtime time.Time) {
	parts := strings.SplitN(v, "\t", 2)
	if len(parts) == 2 {
		size, _ = strconv.ParseInt(parts[0], 10, 64)
		unix, _ := strconv.ParseInt(parts[1], 10, 64)
		mtime = time.Unix(unix, 0).UTC()
	}
	return
}

func matchPath(pattern, path string) bool {
	if strings.Contains(path, pattern) {
		return true
	}
	matched, _ := filepath.Match(pattern, filepath.Base(path))
	return matched
}
