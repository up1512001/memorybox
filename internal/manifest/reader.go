package manifest

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const bufSize = 1 << 20 // 1 MB read/write buffer

// Header holds metadata parsed from manifest comment lines.
type Header struct {
	Message     string
	Snapshot    string
	Updated     int
	Archived    int
	Transferred int64
}

// Reader streams manifest entries without loading the full file into memory.
// Call Next repeatedly until it returns io.EOF, then call Close.
type Reader struct {
	scanner *bufio.Scanner
	Header  Header
	peeked  *Entry // first data entry found while scanning header comments
	done    bool
}

// NewReader wraps r in a streaming manifest reader and immediately
// parses the header comment lines so Header is populated before
// the first call to Next.
func NewReader(r io.Reader) (*Reader, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, bufSize), bufSize)

	mr := &Reader{scanner: scanner}
	if err := mr.readHeader(); err != nil {
		return nil, err
	}
	return mr, nil
}

// Next returns the next Entry or io.EOF when exhausted.
func (r *Reader) Next() (Entry, error) {
	if r.done {
		return Entry{}, io.EOF
	}

	// Return the entry peeked during header parsing first.
	if r.peeked != nil {
		e := *r.peeked
		r.peeked = nil
		return e, nil
	}

	for r.scanner.Scan() {
		line := r.scanner.Text()
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		return ParseLine(line)
	}

	r.done = true
	if err := r.scanner.Err(); err != nil {
		return Entry{}, fmt.Errorf("manifest read: %w", err)
	}
	return Entry{}, io.EOF
}

// Close is a no-op for Reader — the underlying io.Reader is closed by the caller.
func (r *Reader) Close() error { return nil }

// readHeader scans leading comment lines and populates r.Header.
// When it encounters the first non-comment, non-empty line it parses it
// and stores it in r.peeked so Next() returns it on the first call.
func (r *Reader) readHeader() error {
	for r.scanner.Scan() {
		line := r.scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, "#") {
			// First data line — stash for Next().
			e, err := ParseLine(line)
			if err != nil {
				return err
			}
			r.peeked = &e
			return nil
		}
		r.parseHeaderLine(line)
	}
	return r.scanner.Err()
}

func (r *Reader) parseHeaderLine(line string) {
	// Strip leading "# " prefix.
	line = strings.TrimPrefix(line, "# ")
	kv := strings.SplitN(line, ": ", 2)
	if len(kv) != 2 {
		return
	}
	key, val := kv[0], kv[1]
	switch key {
	case "message":
		r.Header.Message = val
	case "snapshot":
		r.Header.Snapshot = val
	case "updated":
		r.Header.Updated, _ = strconv.Atoi(val)
	case "archived":
		r.Header.Archived, _ = strconv.Atoi(val)
	case "transferred":
		r.Header.Transferred, _ = strconv.ParseInt(val, 10, 64)
	}
}
