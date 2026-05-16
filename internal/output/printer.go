// Package output provides the Printer interface and TTY/quiet/JSON implementations.
package output

import "io"

// Printer is the single output abstraction.
// All commands write through Printer; nothing calls fmt.Printf directly.
type Printer interface {
	// Section prints a numbered section header.
	Section(n, total int, name string)
	// Progress overwrites the current line on TTY.
	Progress(msg string)
	// Info prints an informational line.
	Info(msg string)
	// Success prints a success line (green checkmark on TTY).
	Success(msg string)
	// Warn prints a warning line (yellow).
	Warn(msg string)
	// Error prints an error line (red).
	Error(msg string)
	// Table renders a simple ASCII table.
	Table(headers []string, rows [][]string)
	// Flush flushes buffered output.
	Flush()
	// Writer returns the underlying io.Writer for direct use.
	Writer() io.Writer
}
