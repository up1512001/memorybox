package output

import (
	"fmt"
	"io"
	"os"
)

// Quiet is a no-op Printer for --quiet / cron mode.
// Only Error lines are emitted (to stderr).
type Quiet struct{}

// NewQuiet returns a Quiet Printer.
func NewQuiet() *Quiet { return &Quiet{} }

func (q *Quiet) Section(_, _ int, _ string) {}
func (q *Quiet) Progress(_ string)           {}
func (q *Quiet) Info(_ string)               {}
func (q *Quiet) Success(_ string)            {}
func (q *Quiet) Warn(_ string)               {}
func (q *Quiet) Flush()                      {}
func (q *Quiet) Writer() io.Writer           { return os.Stdout }

func (q *Quiet) Error(msg string) {
	fmt.Fprintln(os.Stderr, "ERROR:", msg)
}

func (q *Quiet) Table(_ []string, _ [][]string) {}
