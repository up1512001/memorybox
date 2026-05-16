package rclone

import (
	"testing"
)

func TestIsChanged(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"+ photos/img.jpg", true},   // new file
		{"* docs/notes.txt", true},   // modified
		{"- old/file.txt", false},    // deleted
		{"= same/file.txt", false},   // unchanged
		{"! error/file.txt", false},  // error
		{"", false},                  // empty
		{"+", false},                 // too short
	}
	for _, tc := range cases {
		got := IsChanged(tc.line)
		if got != tc.want {
			t.Errorf("IsChanged(%q) = %v, want %v", tc.line, got, tc.want)
		}
	}
}

func TestIsDeleted(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"- old/file.txt", true},
		{"+ new/file.txt", false},
		{"* changed.txt", false},
		{"= same.txt", false},
		{"", false},
	}
	for _, tc := range cases {
		got := IsDeleted(tc.line)
		if got != tc.want {
			t.Errorf("IsDeleted(%q) = %v, want %v", tc.line, got, tc.want)
		}
	}
}

func TestPath(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{"+ photos/img.jpg", "photos/img.jpg"},
		{"* docs/notes.txt", "docs/notes.txt"},
		{"- old/file.txt", "old/file.txt"},
		{"= same.txt", "same.txt"},
		{"", ""},
		{"+", ""},
	}
	for _, tc := range cases {
		got := Path(tc.line)
		if got != tc.want {
			t.Errorf("Path(%q) = %q, want %q", tc.line, got, tc.want)
		}
	}
}

func TestParseStats_bytes(t *testing.T) {
	output := `
Transferred:   1.5 GiB / 2 GiB, 75%, 10 MiB/s, ETA 5s
Transferred:   42 / 100, 42%
Elapsed time:  30s
`
	stats := ParseStats(output)
	if stats.FilesTransferred != 42 {
		t.Errorf("FilesTransferred = %d, want 42", stats.FilesTransferred)
	}
	// 1.5 GiB ≈ 1.5e9
	if stats.TotalTransferred < 1_400_000_000 || stats.TotalTransferred > 1_600_000_000 {
		t.Errorf("TotalTransferred = %d, want ~1.5e9", stats.TotalTransferred)
	}
}

func TestParseStats_megabytes(t *testing.T) {
	output := "Transferred:   500 MiB / 1 GiB, 50%, 5 MiB/s\nTransferred: 10 / 20, 50%\n"
	stats := ParseStats(output)
	if stats.TotalTransferred < 490_000_000 || stats.TotalTransferred > 510_000_000 {
		t.Errorf("TotalTransferred = %d, want ~500MB", stats.TotalTransferred)
	}
}

func TestParseStats_empty(t *testing.T) {
	stats := ParseStats("")
	if stats == nil {
		t.Error("ParseStats should not return nil")
	}
}
