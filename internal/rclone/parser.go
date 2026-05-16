package rclone

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/up1512001/memorybox/internal/rsync"
)

// --combined output indicators:
//
//	+ new file on destination
//	* file exists on both but differs
//	- file deleted (only with --delete)
//	= identical on both sides
//	! error

// reByteTransfer matches the bytes-transferred line:
//
//	Transferred:   1.5 GiB / 2 GiB, 75%, 10 MiB/s, ETA 5s
//
// Captures: [1]=value, [2]=unit
var reByteTransfer = regexp.MustCompile(
	`Transferred:\s+([\d.]+)\s*(Bytes?|[KMGT]iB|[kKMGT]B)\s*/`)

// reFileCount matches the file-count line:
//
//	Transferred:   42 / 100, 42%
var reFileCount = regexp.MustCompile(`Transferred:\s+(\d+)\s*/\s*\d+`)

// IsChanged reports whether a --combined line represents a new or modified file.
func IsChanged(line string) bool {
	return len(line) >= 2 && (line[0] == '+' || line[0] == '*')
}

// IsDeleted reports whether a --combined line represents a deleted file.
func IsDeleted(line string) bool {
	return len(line) >= 2 && line[0] == '-'
}

// Path extracts the file path from a --combined line (skips the leading "X " prefix).
func Path(line string) string {
	if len(line) < 2 {
		return ""
	}
	return strings.TrimSpace(line[2:])
}

// ParseStats extracts aggregate transfer numbers from rclone --stats output.
// Rclone stats look like:
//
//	Transferred:   1.234 GiB / 2 GiB, 62%, 10 MiB/s, ETA 1m2s
//	Transferred:   42 / 100, 42%
func ParseStats(output string) *rsync.Stats {
	s := &rsync.Stats{}

	// File count — "Transferred: N / total, pct%"
	if m := reFileCount.FindAllStringSubmatch(output, -1); len(m) > 0 {
		last := m[len(m)-1]
		s.FilesTransferred, _ = strconv.ParseInt(last[1], 10, 64)
	}

	// Bytes transferred — "Transferred: X Unit / Y Unit, ..." (keep last match)
	if m := reByteTransfer.FindAllStringSubmatch(output, -1); len(m) > 0 {
		last := m[len(m)-1]
		val, _ := strconv.ParseFloat(last[1], 64)
		unit := strings.ToUpper(last[2])
		switch {
		case strings.HasPrefix(unit, "G"):
			s.TotalTransferred = int64(val * 1e9)
		case strings.HasPrefix(unit, "M"):
			s.TotalTransferred = int64(val * 1e6)
		case strings.HasPrefix(unit, "K"):
			s.TotalTransferred = int64(val * 1e3)
		default:
			s.TotalTransferred = int64(val)
		}
	}
	return s
}
