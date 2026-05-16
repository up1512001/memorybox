package gitignore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectExcludes_basic(t *testing.T) {
	root := t.TempDir()

	write(t, filepath.Join(root, ".gitignore"), `
# comment
node_modules
dist/
*.log
!important.log
`)

	got := CollectExcludes(root, 2)
	want := map[string]bool{
		"node_modules": true,
		"dist/":        true,
		"*.log":        true,
	}
	for _, p := range got {
		if !want[p] {
			t.Errorf("unexpected pattern %q", p)
		}
		delete(want, p)
	}
	for p := range want {
		t.Errorf("missing expected pattern %q", p)
	}
}

func TestCollectExcludes_nested(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	os.MkdirAll(sub, 0o755)

	write(t, filepath.Join(root, ".gitignore"), "vendor\n")
	write(t, filepath.Join(sub, ".gitignore"), "build\nvendor\n") // vendor is a duplicate

	got := CollectExcludes(root, 2)
	seen := make(map[string]int)
	for _, p := range got {
		seen[p]++
	}
	if seen["vendor"] != 1 {
		t.Errorf("vendor should appear exactly once, got %d", seen["vendor"])
	}
	if seen["build"] != 1 {
		t.Errorf("build should appear once, got %d", seen["build"])
	}
}

func TestCollectExcludes_missingDir(t *testing.T) {
	got := CollectExcludes("/nonexistent/path/xyz", 2)
	if got != nil {
		t.Errorf("expected nil for missing dir, got %v", got)
	}
}

func TestCollectExcludes_stripsLeadingSlash(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".gitignore"), "/dist\n")

	got := CollectExcludes(root, 1)
	if len(got) != 1 || got[0] != "dist" {
		t.Errorf("expected [\"dist\"], got %v", got)
	}
}

func TestCollectExcludes_depthLimit(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b", "c")
	os.MkdirAll(deep, 0o755)
	write(t, filepath.Join(deep, ".gitignore"), "deep_pattern\n")

	got := CollectExcludes(root, 1) // maxDepth=1 shouldn't reach a/b/c
	for _, p := range got {
		if p == "deep_pattern" {
			t.Errorf("depth limit not respected: found deep_pattern at depth >1")
		}
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
