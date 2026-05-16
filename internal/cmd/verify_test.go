package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/up1512001/memorybox/internal/app"
	"github.com/up1512001/memorybox/internal/config"
	"github.com/up1512001/memorybox/internal/manifest"
	"github.com/up1512001/memorybox/internal/output"
	"github.com/up1512001/memorybox/internal/snapshot"
)

func newVerifyApp(t *testing.T) (*app.App, string) {
	t.Helper()
	root := t.TempDir()
	manifestDir := filepath.Join(root, "manifests")
	archiveDir := filepath.Join(root, "archive")
	backupDir := filepath.Join(root, "backup-current")
	os.MkdirAll(manifestDir, 0o755)
	os.MkdirAll(archiveDir, 0o755)
	os.MkdirAll(backupDir, 0o755)

	a := &app.App{
		Cfg: config.Config{
			Drive: config.DriveConfig{
				MountPath:   root,
				BackupDir:   backupDir,
				ManifestDir: manifestDir,
				ArchiveDir:  archiveDir,
			},
		},
		Printer: output.NewQuiet(),
		Store:   snapshot.NewStore(manifestDir, archiveDir),
	}
	return a, backupDir
}

// writeManifestWithEntry creates a snapshot manifest containing one entry.
func writeManifestWithEntry(t *testing.T, a *app.App, relPath string, size int64, mtime time.Time) {
	t.Helper()
	ctx := context.Background()
	snap, err := a.Store.Create(ctx, snapshot.SnapshotMeta{Label: "test"})
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}

	w, closeW, err := a.Store.ManifestWriter(snap.Key)
	if err != nil {
		t.Fatalf("manifest writer: %v", err)
	}
	defer closeW()

	w.WriteHeader(manifest.Header{Message: "test", Snapshot: snap.Key})
	w.Write(manifest.Entry{
		Path:  relPath,
		Size:  size,
		MTime: mtime.UTC(),
		Mode:  0o644,
	})
}

func stubCmd(a *app.App) *cobra.Command {
	return &cobra.Command{Use: "stub"}
}

func TestVerify_clean(t *testing.T) {
	a, backupDir := newVerifyApp(t)

	mtime := time.Unix(1700000000, 0).UTC()
	content := []byte("hello world")
	relPath := "photos/img.jpg"

	os.MkdirAll(filepath.Join(backupDir, "photos"), 0o755)
	if err := os.WriteFile(filepath.Join(backupDir, relPath), content, 0o644); err != nil {
		t.Fatal(err)
	}
	// Set mtime to match manifest.
	os.Chtimes(filepath.Join(backupDir, relPath), mtime, mtime)

	writeManifestWithEntry(t, a, relPath, int64(len(content)), mtime)

	cmd := stubCmd(a)
	if err := runVerify(cmd, a); err != nil {
		t.Errorf("expected clean verify, got error: %v", err)
	}
}

func TestVerify_missingFile(t *testing.T) {
	a, _ := newVerifyApp(t)

	mtime := time.Unix(1700000000, 0).UTC()
	writeManifestWithEntry(t, a, "docs/missing.txt", 100, mtime)

	cmd := stubCmd(a)
	err := runVerify(cmd, a)
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestVerify_sizeDiverged(t *testing.T) {
	a, backupDir := newVerifyApp(t)

	mtime := time.Unix(1700000000, 0).UTC()
	relPath := "docs/note.txt"
	os.MkdirAll(filepath.Join(backupDir, "docs"), 0o755)
	os.WriteFile(filepath.Join(backupDir, relPath), []byte("short"), 0o644)
	os.Chtimes(filepath.Join(backupDir, relPath), mtime, mtime)

	writeManifestWithEntry(t, a, relPath, 9999, mtime) // wrong size in manifest

	cmd := stubCmd(a)
	if err := runVerify(cmd, a); err == nil {
		t.Error("expected error for size divergence, got nil")
	}
}

func TestVerify_noSnapshots(t *testing.T) {
	a, _ := newVerifyApp(t)
	cmd := stubCmd(a)
	if err := runVerify(cmd, a); err == nil {
		t.Error("expected error when no snapshots exist, got nil")
	}
}
