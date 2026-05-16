package manifest

import (
	"bufio"
	"fmt"
	"io"
)

// Writer streams sorted manifest entries to an underlying writer.
// Call Flush then Close when done — omitting Flush loses buffered data.
type Writer struct {
	bw *bufio.Writer
}

// NewWriter wraps w in a buffered manifest writer.
func NewWriter(w io.Writer) *Writer {
	return &Writer{bw: bufio.NewWriterSize(w, bufSize)}
}

// WriteHeader emits the manifest metadata comment block.
// Must be called before any Write calls.
func (w *Writer) WriteHeader(h Header) error {
	lines := []string{
		fmt.Sprintf("# message: %s", h.Message),
		fmt.Sprintf("# snapshot: %s", h.Snapshot),
		fmt.Sprintf("# updated: %d", h.Updated),
		fmt.Sprintf("# archived: %d", h.Archived),
		fmt.Sprintf("# transferred: %d", h.Transferred),
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(w.bw, l); err != nil {
			return err
		}
	}
	return nil
}

// Write appends one entry to the manifest.
func (w *Writer) Write(e Entry) error {
	_, err := fmt.Fprintln(w.bw, e.Line())
	return err
}

// Flush flushes buffered data to the underlying writer.
func (w *Writer) Flush() error { return w.bw.Flush() }
