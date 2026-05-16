// Package color provides named ANSI style functions with TTY detection.
// Call Init once at startup; all downstream code uses the Style variables.
package color

import (
	"os"

	"golang.org/x/term"
)

// Style wraps a string with ANSI escape codes when color is enabled,
// or returns the string unchanged when color is disabled.
type Style func(string) string

// Named style variables. Always call Init before using these.
var (
	Bold   Style
	Dim    Style
	Red    Style
	Green  Style
	Yellow Style
	Blue   Style
	Cyan   Style
	Reset  Style

	enabled bool
)

// Init configures color output. Must be called once before any output.
// Priority (highest to lowest):
//  1. forceDisable = true → always off
//  2. NO_COLOR env var set (any value) → off
//  3. TERM=dumb → off
//  4. stdout is not a TTY → off
//  5. otherwise → on
func Init(forceDisable bool) {
	enabled = !forceDisable &&
		os.Getenv("NO_COLOR") == "" &&
		os.Getenv("TERM") != "dumb" &&
		term.IsTerminal(int(os.Stdout.Fd()))

	if enabled {
		Bold   = wrap("\033[1m", "\033[0m")
		Dim    = wrap("\033[2m", "\033[0m")
		Red    = wrap("\033[31m", "\033[0m")
		Green  = wrap("\033[32m", "\033[0m")
		Yellow = wrap("\033[33m", "\033[0m")
		Blue   = wrap("\033[34m", "\033[0m")
		Cyan   = wrap("\033[36m", "\033[0m")
		Reset  = func(s string) string { return s }
	} else {
		id := func(s string) string { return s }
		Bold, Dim, Red, Green, Yellow, Blue, Cyan, Reset = id, id, id, id, id, id, id, id
	}
}

// IsEnabled reports whether color output is currently active.
func IsEnabled() bool { return enabled }

func wrap(open, close string) Style {
	return func(s string) string { return open + s + close }
}
