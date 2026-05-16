// Package gitignore walks a source tree and collects rsync-compatible
// exclude patterns derived from .gitignore files it finds.
package gitignore

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// CollectExcludes walks root up to maxDepth directories deep, reads every
// .gitignore it finds, and returns a deduplicated list of rsync --exclude
// patterns. Patterns that are already absolute (start with /) are kept as-is;
// relative patterns are returned as-is for rsync to match anywhere in the tree.
func CollectExcludes(root string, maxDepth int) []string {
	if maxDepth <= 0 {
		maxDepth = 4
	}
	seen := make(map[string]bool)
	var out []string

	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		giPath := filepath.Join(dir, ".gitignore")
		if patterns, err := parse(giPath); err == nil {
			for _, p := range patterns {
				if !seen[p] {
					seen[p] = true
					out = append(out, p)
				}
			}
		}
		if depth >= maxDepth {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			walk(filepath.Join(dir, e.Name()), depth+1)
		}
	}

	walk(root, 0)
	return out
}

// parse reads a .gitignore file and returns non-empty, non-comment lines
// normalised to rsync exclude syntax.
func parse(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Negation patterns (!foo) are not supported by rsync exclude — skip.
		if strings.HasPrefix(line, "!") {
			continue
		}
		// Strip leading slash — rsync anchors differently from gitignore.
		line = strings.TrimPrefix(line, "/")
		if line != "" {
			out = append(out, line)
		}
	}
	return out, scanner.Err()
}
