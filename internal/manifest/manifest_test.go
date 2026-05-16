package manifest_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/up1512001/memorybox/internal/manifest"
)

// ── ParseLine ──────────────────────────────────────────────────────────────

var parseLineTests = []struct {
	name    string
	line    string
	wantErr bool
	want    manifest.Entry
}{
	{
		name: "simple path",
		line: "33188\t1024\t1716000000\tDocuments/report.pdf",
		want: manifest.Entry{
			Mode:  33188,
			Size:  1024,
			MTime: time.Unix(1716000000, 0).UTC(),
			Path:  "Documents/report.pdf",
		},
	},
	{
		name: "path with spaces — the critical case",
		line: "33188\t5678\t1716000001\tiCloud Drive/My Project/Design Doc.pages",
		want: manifest.Entry{
			Mode:  33188,
			Size:  5678,
			MTime: time.Unix(1716000001, 0).UTC(),
			Path:  "iCloud Drive/My Project/Design Doc.pages",
		},
	},
	{
		name: "zero size",
		line: "33188\t0\t1716000002\tDesktop/.DS_Store",
		want: manifest.Entry{
			Mode:  33188,
			Size:  0,
			MTime: time.Unix(1716000002, 0).UTC(),
			Path:  "Desktop/.DS_Store",
		},
	},
	{
		name: "local sites path with spaces",
		line: "33261\t4096\t1716000003\tLocal Sites/my-client-site/wp-content/plugins/plugin.php",
		want: manifest.Entry{
			Mode:  33261,
			Size:  4096,
			MTime: time.Unix(1716000003, 0).UTC(),
			Path:  "Local Sites/my-client-site/wp-content/plugins/plugin.php",
		},
	},
	{
		name:    "too few fields",
		line:    "33188\t1024",
		wantErr: true,
	},
	{
		name:    "empty line",
		line:    "",
		wantErr: true,
	},
	{
		name:    "bad mode",
		line:    "notanumber\t1024\t1716000000\tfile.txt",
		wantErr: true,
	},
}

func TestParseLine(t *testing.T) {
	for _, tc := range parseLineTests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := manifest.ParseLine(tc.line)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Path != tc.want.Path {
				t.Errorf("Path: got %q, want %q", got.Path, tc.want.Path)
			}
			if got.Size != tc.want.Size {
				t.Errorf("Size: got %d, want %d", got.Size, tc.want.Size)
			}
			if !got.MTime.Equal(tc.want.MTime) {
				t.Errorf("MTime: got %v, want %v", got.MTime, tc.want.MTime)
			}
		})
	}
}

// ── Round-trip: Entry.Line() → ParseLine() ─────────────────────────────────

func TestEntryLineRoundTrip(t *testing.T) {
	paths := []string{
		"simple.txt",
		"iCloud Drive/My File With Spaces.pdf",
		"Local Sites/client site/file.php",
		"Developer/my project/src/main.go",
		"Documents/налоговая.pdf", // Unicode
	}
	for _, p := range paths {
		e := manifest.Entry{
			Path:  p,
			Size:  12345,
			MTime: time.Unix(1716000000, 0).UTC(),
			Mode:  fs.FileMode(0o644),
		}
		got, err := manifest.ParseLine(e.Line())
		if err != nil {
			t.Fatalf("path %q: ParseLine(%q): %v", p, e.Line(), err)
		}
		if got.Path != p {
			t.Errorf("path %q: round-trip got %q", p, got.Path)
		}
	}
}

// ── Writer + Reader round-trip ─────────────────────────────────────────────

func TestWriterReaderRoundTrip(t *testing.T) {
	entries := []manifest.Entry{
		{Path: "a/file.txt", Size: 100, MTime: time.Unix(1000, 0).UTC(), Mode: 0o644},
		{Path: "b/another file.txt", Size: 200, MTime: time.Unix(2000, 0).UTC(), Mode: 0o644},
		{Path: "iCloud Drive/Doc.pages", Size: 300, MTime: time.Unix(3000, 0).UTC(), Mode: 0o644},
	}

	var buf bytes.Buffer
	w := manifest.NewWriter(&buf)
	if err := w.WriteHeader(manifest.Header{
		Message:     "test backup",
		Snapshot:    "2026-05-16T14-30-00Z",
		Updated:     3,
		Archived:    0,
		Transferred: 600,
	}); err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if err := w.Write(e); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	r, err := manifest.NewReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if r.Header.Message != "test backup" {
		t.Errorf("Header.Message: got %q", r.Header.Message)
	}
	if r.Header.Updated != 3 {
		t.Errorf("Header.Updated: got %d", r.Header.Updated)
	}

	var got []manifest.Entry
	for {
		e, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, e)
	}

	if len(got) != len(entries) {
		t.Fatalf("got %d entries, want %d", len(got), len(entries))
	}
	for i, want := range entries {
		if got[i].Path != want.Path {
			t.Errorf("[%d] Path: got %q, want %q", i, got[i].Path, want.Path)
		}
		if got[i].Size != want.Size {
			t.Errorf("[%d] Size: got %d, want %d", i, got[i].Size, want.Size)
		}
	}
}

// ── Merge ─────────────────────────────────────────────────────────────────

func manifestFromLines(lines ...string) *manifest.Reader {
	r, _ := manifest.NewReader(strings.NewReader(strings.Join(lines, "\n") + "\n"))
	return r
}

func entryLine(path string, size int64) string {
	return manifest.Entry{
		Path:  path,
		Size:  size,
		MTime: time.Unix(1000, 0).UTC(),
		Mode:  0o644,
	}.Line()
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name         string
		left         []string
		right        []string
		wantAdded    []string
		wantDeleted  []string
		wantModified []string
		wantSame     []string
	}{
		{
			name:      "added only",
			left:      []string{},
			right:     []string{entryLine("a.txt", 100)},
			wantAdded: []string{"a.txt"},
		},
		{
			name:       "deleted only",
			left:       []string{entryLine("a.txt", 100)},
			right:      []string{},
			wantDeleted: []string{"a.txt"},
		},
		{
			name:     "unchanged",
			left:     []string{entryLine("a.txt", 100)},
			right:    []string{entryLine("a.txt", 100)},
			wantSame: []string{"a.txt"},
		},
		{
			name:         "modified (size changed)",
			left:         []string{entryLine("a.txt", 100)},
			right:        []string{entryLine("a.txt", 200)},
			wantModified: []string{},
			// Both left and right non-nil with same path = modified candidate
			// (actual modified detection is in diff package, merge just pairs them)
		},
		{
			name: "mixed with spaces in paths",
			left: []string{
				entryLine("iCloud Drive/Old File.pages", 500),
				entryLine("iCloud Drive/Unchanged.pdf", 100),
			},
			right: []string{
				entryLine("iCloud Drive/New File.pages", 600),
				entryLine("iCloud Drive/Unchanged.pdf", 100),
			},
			wantAdded:   []string{"iCloud Drive/New File.pages"},
			wantDeleted: []string{"iCloud Drive/Old File.pages"},
			wantSame:    []string{"iCloud Drive/Unchanged.pdf"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			left := manifestFromLines(tc.left...)
			right := manifestFromLines(tc.right...)

			out := make(chan manifest.MergeEvent, 64)
			errCh := make(chan error, 1)
			go func() {
				errCh <- manifest.Merge(context.Background(), left, right, out)
			}()

			var added, deleted, both []string
			for ev := range out {
				switch {
				case ev.Left == nil && ev.Right != nil:
					added = append(added, ev.Right.Path)
				case ev.Left != nil && ev.Right == nil:
					deleted = append(deleted, ev.Left.Path)
				default:
					both = append(both, ev.Left.Path)
				}
			}

			if err := <-errCh; err != nil {
				t.Fatal(err)
			}

			if len(tc.wantAdded) > 0 {
				if len(added) != len(tc.wantAdded) {
					t.Errorf("added: got %v, want %v", added, tc.wantAdded)
				}
			}
			if len(tc.wantDeleted) > 0 {
				if len(deleted) != len(tc.wantDeleted) {
					t.Errorf("deleted: got %v, want %v", deleted, tc.wantDeleted)
				}
			}
			for _, p := range tc.wantSame {
				found := false
				for _, b := range both {
					if b == p {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected %q in both-sides events, got: %v", p, both)
				}
			}
		})
	}
}

// ── Benchmark: 1M entry merge ──────────────────────────────────────────────

func BenchmarkMerge1M(b *testing.B) {
	const N = 1_000_000

	makeReader := func() *manifest.Reader {
		var sb strings.Builder
		for i := 0; i < N; i++ {
			e := manifest.Entry{
				Path:  fmt.Sprintf("Documents/file_%07d.pdf", i),
				Size:  int64(i),
				MTime: time.Unix(int64(i), 0).UTC(),
				Mode:  0o644,
			}
			sb.WriteString(e.Line())
			sb.WriteByte('\n')
		}
		r, _ := manifest.NewReader(strings.NewReader(sb.String()))
		return r
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		left := makeReader()
		right := makeReader()
		out := make(chan manifest.MergeEvent, 4096)
		go func() {
			manifest.Merge(context.Background(), left, right, out)
		}()
		for range out {
		}
	}
}
