// Package testutil provides shared test helpers.
package testutil

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/up1512001/memorybox/internal/rsync"
)

// MockRsyncRunner implements rsync.Runner using Go file operations.
// It copies all files from source to dest (last two args), emitting
// ">f+++++++++ <path>" lines so callers see realistic change counts.
type MockRsyncRunner struct {
	// LastSrc and LastDst are set after each Run call, useful in assertions.
	LastSrc string
	LastDst string
}

func (m *MockRsyncRunner) Run(_ context.Context, args []string, lines chan<- rsync.Line) (*rsync.Stats, error) {
	defer close(lines)

	if len(args) < 2 {
		return &rsync.Stats{}, nil
	}

	src := strings.TrimSuffix(args[len(args)-2], "/")
	dst := strings.TrimSuffix(args[len(args)-1], "/")
	m.LastSrc = src
	m.LastDst = dst

	if _, err := os.Stat(src); os.IsNotExist(err) {
		return &rsync.Stats{}, nil
	}
	os.MkdirAll(dst, 0o755)

	var count int64
	filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, rel)
		os.MkdirAll(filepath.Dir(dstPath), 0o755)

		if copyFile(path, dstPath) == nil {
			lines <- rsync.Line{Raw: ">f+++++++++ " + rel}
			count++
		}
		return nil
	})

	return &rsync.Stats{FilesTransferred: count, TotalTransferred: count * 1024}, nil
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
	_, err = io.Copy(out, in)
	return err
}
