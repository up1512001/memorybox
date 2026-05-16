package manifest

import (
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"time"
)

// ErrMalformedEntry is returned when a manifest line cannot be parsed.
var ErrMalformedEntry = errors.New("malformed manifest entry")

// Entry is one file record in a manifest.
// Wire format (tab-delimited): <mode>\t<size>\t<mtime-unix>\t<path>
// Path is always the last field so filenames with tabs (extremely rare) are safe,
// and — critically — filenames with spaces are handled correctly.
type Entry struct {
	Path  string
	Size  int64
	MTime time.Time
	Mode  fs.FileMode
}

// Line serialises Entry to the single-line manifest format.
func (e Entry) Line() string {
	return fmt.Sprintf("%d\t%d\t%d\t%s", e.Mode, e.Size, e.MTime.Unix(), e.Path)
}

// ParseLine deserialises one manifest data line into an Entry.
// Returns ErrMalformedEntry if the line is invalid.
func ParseLine(line string) (Entry, error) {
	parts := strings.SplitN(line, "\t", 4)
	if len(parts) != 4 {
		return Entry{}, fmt.Errorf("%w: want 4 tab-delimited fields, got %d in %q",
			ErrMalformedEntry, len(parts), line)
	}

	rawMode, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return Entry{}, fmt.Errorf("%w: mode %q: %v", ErrMalformedEntry, parts[0], err)
	}

	size, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return Entry{}, fmt.Errorf("%w: size %q: %v", ErrMalformedEntry, parts[1], err)
	}

	mtimeUnix, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return Entry{}, fmt.Errorf("%w: mtime %q: %v", ErrMalformedEntry, parts[2], err)
	}

	return Entry{
		Mode:  fs.FileMode(rawMode),
		Size:  size,
		MTime: time.Unix(mtimeUnix, 0).UTC(),
		Path:  parts[3],
	}, nil
}
