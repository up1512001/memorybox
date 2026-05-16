package rsync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckVersion_notFound(t *testing.T) {
	warn := CheckVersion("/nonexistent/rsync-binary-xyz")
	if warn == "" {
		t.Error("expected warning for missing rsync, got empty string")
	}
}

func TestCheckVersion_fakeModernRsync(t *testing.T) {
	// Write a fake rsync that prints a modern version string and exits 0.
	dir := t.TempDir()
	script := filepath.Join(dir, "rsync")
	os.WriteFile(script, []byte("#!/bin/sh\necho 'rsync  version 3.3.0  protocol version 31'\n"), 0o755)

	warn := CheckVersion(script)
	if warn != "" {
		t.Errorf("expected no warning for modern rsync, got: %s", warn)
	}
}

func TestCheckVersion_fakeOpenRsync(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "rsync")
	os.WriteFile(script, []byte("#!/bin/sh\necho 'openrsync: protocol version 29, rsync version 2.6.9 compatible'\n"), 0o755)

	warn := CheckVersion(script)
	if warn == "" {
		t.Error("expected warning for openrsync, got empty string")
	}
}

func TestCheckVersion_fakeOldRsync(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "rsync")
	os.WriteFile(script, []byte("#!/bin/sh\necho 'rsync version 2.6.9'\n"), 0o755)

	warn := CheckVersion(script)
	if warn == "" {
		t.Error("expected warning for rsync v2, got empty string")
	}
}
