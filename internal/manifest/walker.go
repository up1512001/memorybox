package manifest

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
)

// Walk traverses root and returns all file entries sorted by path,
// using a bounded worker pool to parallelise os.Lstat calls.
// This is significantly faster than find -exec stat {} \; for large trees.
func Walk(ctx context.Context, root string) ([]Entry, error) {
	type work struct {
		rel  string
		info fs.DirEntry
	}

	workers := runtime.NumCPU()
	if workers < 2 {
		workers = 2
	}

	workCh := make(chan work, 4096)
	var (
		mu      sync.Mutex
		entries []Entry
		walkErr error
	)

	// Worker pool: stat each file and accumulate entries.
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for w := range workCh {
				if ctx.Err() != nil {
					return
				}
				info, err := os.Lstat(filepath.Join(root, w.rel))
				if err != nil {
					continue // skip unreadable files
				}
				e := Entry{
					Path:  w.rel,
					Size:  info.Size(),
					MTime: info.ModTime().UTC(),
					Mode:  info.Mode(),
				}
				mu.Lock()
				entries = append(entries, e)
				mu.Unlock()
			}
		}()
	}

	// Feed worker pool via WalkDir (no Lstat per entry in the walker itself).
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		rel, _ := filepath.Rel(root, path)
		workCh <- work{rel: rel, info: d}
		return nil
	})
	close(workCh)
	wg.Wait()

	if err != nil && err != ctx.Err() {
		walkErr = err
	}
	if walkErr != nil {
		return nil, walkErr
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	return entries, nil
}
