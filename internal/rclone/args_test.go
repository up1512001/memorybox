package rclone

import (
	"strings"
	"testing"
)

func TestBuildSyncArgs_basic(t *testing.T) {
	opts := SyncOpts{
		Source:      "/Users/me/Pictures",
		Destination: "r2:bucket/membox/photos",
	}
	args := BuildSyncArgs(opts)

	checkArg(t, args, "sync")
	checkArg(t, args, "--combined")
	checkArg(t, args, "/Users/me/Pictures")
	checkArg(t, args, "r2:bucket/membox/photos")
}

func TestBuildSyncArgs_dryRun(t *testing.T) {
	args := BuildSyncArgs(SyncOpts{Source: "src", Destination: "dst", DryRun: true})
	checkArg(t, args, "--dry-run")
}

func TestBuildSyncArgs_excludes(t *testing.T) {
	args := BuildSyncArgs(SyncOpts{
		Source:      "src",
		Destination: "dst",
		Excludes:    []string{"node_modules", "*.log"},
	})
	checkArg(t, args, "node_modules")
	checkArg(t, args, "*.log")
	// --exclude flag must precede each pattern
	for i, a := range args {
		if a == "node_modules" && (i == 0 || args[i-1] != "--exclude") {
			t.Error("node_modules not preceded by --exclude")
		}
	}
}

func TestBuildSyncArgs_bwlimit(t *testing.T) {
	args := BuildSyncArgs(SyncOpts{Source: "s", Destination: "d", BwLimit: 5000})
	checkArg(t, args, "--bwlimit")
	checkArg(t, args, "4M") // 5000/1024 ≈ 4M
}

func TestBuildCopyArgs(t *testing.T) {
	args := BuildCopyArgs(CopyOpts{Source: "r2:bucket/file.txt", Destination: "/tmp/restore"})
	if args[0] != "copy" {
		t.Errorf("first arg should be 'copy', got %q", args[0])
	}
	checkArg(t, args, "r2:bucket/file.txt")
	checkArg(t, args, "/tmp/restore")
}

func TestRemoteManifestPath(t *testing.T) {
	got := RemoteManifestPath("r2:bucket/membox", "2026-05-16T10-00-00Z")
	want := "r2:bucket/membox/manifests/2026-05-16T10-00-00Z.manifest"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func checkArg(t *testing.T, args []string, want string) {
	t.Helper()
	for _, a := range args {
		if a == want || strings.Contains(a, want) {
			return
		}
	}
	t.Errorf("arg %q not found in %v", want, args)
}
