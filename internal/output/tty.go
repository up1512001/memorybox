package output

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/up1512001/memorybox/internal/color"
)

// TTY is the color-aware Printer for interactive terminal use.
type TTY struct {
	w io.Writer
}

// NewTTY returns a TTY Printer writing to w (typically os.Stdout).
func NewTTY(w io.Writer) *TTY {
	if w == nil {
		w = os.Stdout
	}
	return &TTY{w: w}
}

func (t *TTY) Section(n, total int, name string) {
	fmt.Fprintf(t.w, "\n%s\n",
		color.Cyan(fmt.Sprintf("[%d/%d] %s", n, total, name)),
	)
}

func (t *TTY) Progress(msg string) {
	fmt.Fprintf(t.w, "\r  %s", msg)
}

func (t *TTY) Info(msg string) {
	fmt.Fprintf(t.w, "  %s\n", msg)
}

func (t *TTY) Success(msg string) {
	fmt.Fprintf(t.w, "  %s %s\n", color.Green("✓"), msg)
}

func (t *TTY) Warn(msg string) {
	fmt.Fprintf(t.w, "  %s %s\n", color.Yellow("⚠"), msg)
}

func (t *TTY) Error(msg string) {
	fmt.Fprintf(t.w, "  %s %s\n", color.Red("✗"), msg)
}

func (t *TTY) Table(headers []string, rows [][]string) {
	tw := tabwriter.NewWriter(t.w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	fmt.Fprintln(tw, strings.Repeat("─", 60))
	for _, row := range rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	tw.Flush()
}

func (t *TTY) Flush() {}

func (t *TTY) Writer() io.Writer { return t.w }
