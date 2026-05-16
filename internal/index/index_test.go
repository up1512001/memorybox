package index

import (
	"io"
	"testing"
	"time"
)

func TestIndexFromReader_searchFound(t *testing.T) {
	idx, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer idx.Close()

	mtime := time.Unix(1700000000, 0).UTC()
	entries := []struct{ path string; size int64 }{
		{"photos/img.jpg", 1024},
		{"docs/notes.txt", 512},
		{"dev/main.go", 2048},
	}
	i := 0
	err = idx.IndexFromReader("2026-05-16T10-00-00Z", func() (string, int64, time.Time, error) {
		if i >= len(entries) {
			return "", 0, time.Time{}, io.EOF
		}
		e := entries[i]
		i++
		return e.path, e.size, mtime, nil
	})
	if err != nil {
		t.Fatalf("IndexFromReader: %v", err)
	}

	out := make(chan Entry, 10)
	if err := idx.Search("img.jpg", out); err != nil {
		t.Fatalf("Search: %v", err)
	}

	var results []Entry
	for e := range out {
		results = append(results, e)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Path != "photos/img.jpg" {
		t.Errorf("unexpected path %q", results[0].Path)
	}
	if results[0].Size != 1024 {
		t.Errorf("unexpected size %d", results[0].Size)
	}
	if !results[0].MTime.Equal(mtime) {
		t.Errorf("mtime mismatch: got %v want %v", results[0].MTime, mtime)
	}
}

func TestIndexFromReader_searchNotFound(t *testing.T) {
	idx, _ := Open(t.TempDir())
	defer idx.Close()

	i := 0
	data := []string{"photos/a.jpg"}
	idx.IndexFromReader("snap1", func() (string, int64, time.Time, error) {
		if i >= len(data) {
			return "", 0, time.Time{}, io.EOF
		}
		p := data[i]; i++
		return p, 100, time.Now(), nil
	})

	out := make(chan Entry, 10)
	idx.Search("nonexistent.txt", out)
	if len(out) != 0 {
		t.Errorf("expected 0 results for nonexistent pattern")
	}
}

func TestIndexFromReader_multipleSnapshots(t *testing.T) {
	idx, _ := Open(t.TempDir())
	defer idx.Close()

	for _, snap := range []string{"snap1", "snap2"} {
		s := snap
		i := 0
		idx.IndexFromReader(s, func() (string, int64, time.Time, error) {
			if i > 0 {
				return "", 0, time.Time{}, io.EOF
			}
			i++
			return "docs/file.txt", 100, time.Now(), nil
		})
	}

	out := make(chan Entry, 10)
	idx.Search("file.txt", out)
	var results []Entry
	for e := range out {
		results = append(results, e)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results (one per snapshot), got %d", len(results))
	}
}

func TestDeleteSnapshot(t *testing.T) {
	idx, _ := Open(t.TempDir())
	defer idx.Close()

	i := 0
	idx.IndexFromReader("snap1", func() (string, int64, time.Time, error) {
		if i > 0 {
			return "", 0, time.Time{}, io.EOF
		}
		i++
		return "a.txt", 1, time.Now(), nil
	})

	if err := idx.DeleteSnapshot("snap1"); err != nil {
		t.Fatalf("DeleteSnapshot: %v", err)
	}

	out := make(chan Entry, 10)
	idx.Search("a.txt", out)
	if len(out) != 0 {
		t.Errorf("expected 0 results after deletion")
	}
}

func TestStats(t *testing.T) {
	idx, _ := Open(t.TempDir())
	defer idx.Close()

	for snap, count := range map[string]int{"s1": 3, "s2": 2} {
		n := 0
		c := count
		idx.IndexFromReader(snap, func() (string, int64, time.Time, error) {
			if n >= c {
				return "", 0, time.Time{}, io.EOF
			}
			p := snap + "/" + string(rune('a'+n))
			n++
			return p, 10, time.Now(), nil
		})
	}

	snaps, entries, err := idx.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if snaps != 2 {
		t.Errorf("expected 2 snapshots, got %d", snaps)
	}
	if entries != 5 {
		t.Errorf("expected 5 entries, got %d", entries)
	}
}

func TestGlobPattern(t *testing.T) {
	idx, _ := Open(t.TempDir())
	defer idx.Close()

	i := 0
	files := []string{"photos/img.jpg", "photos/thumb.png", "docs/notes.txt"}
	idx.IndexFromReader("snap1", func() (string, int64, time.Time, error) {
		if i >= len(files) {
			return "", 0, time.Time{}, io.EOF
		}
		p := files[i]; i++
		return p, 100, time.Now(), nil
	})

	out := make(chan Entry, 10)
	idx.Search("*.jpg", out)
	var results []Entry
	for e := range out {
		results = append(results, e)
	}
	if len(results) != 1 || results[0].Path != "photos/img.jpg" {
		t.Errorf("glob *.jpg: expected photos/img.jpg, got %v", results)
	}
}
