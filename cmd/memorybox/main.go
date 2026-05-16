package main

import (
	"fmt"
	"os"

	"github.com/up1512001/memorybox/internal/app"
	"github.com/up1512001/memorybox/internal/cmd"
)

// Injected at build time via -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	a := app.New(app.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	})

	root := cmd.NewRootCmd(a)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
