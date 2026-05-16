package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/up1512001/memorybox/internal/app"
	"github.com/up1512001/memorybox/internal/color"
	diffpkg "github.com/up1512001/memorybox/internal/diff"
	"github.com/up1512001/memorybox/internal/snapshot"
)

func newDiffCmd(a *app.App) *cobra.Command {
	var (
		statOnly   bool
		nameOnly   bool
		nameStatus bool
	)

	cmd := &cobra.Command{
		Use:   "diff [snapshot]",
		Short: "Show what changed between snapshots (like git diff)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := ""
			if len(args) > 0 {
				target = args[0]
			}
			return runDiff(cmd.Context(), a, target, statOnly, nameOnly, nameStatus)
		},
	}

	cmd.Flags().BoolVar(&statOnly, "stat", false, "show counts only, no file list")
	cmd.Flags().BoolVar(&nameOnly, "name-only", false, "list changed file paths")
	cmd.Flags().BoolVar(&nameStatus, "name-status", false, "A/D/M prefix + path")

	return cmd
}

func runDiff(ctx context.Context, a *app.App, target string, statOnly, nameOnly, nameStatus bool) error {
	if err := a.CheckDrive(0); err != nil {
		return err
	}

	// Resolve target snapshot.
	var toSnap snapshot.Snapshot
	var err error
	if target == "" {
		toSnap, err = a.Store.Latest(ctx)
	} else {
		toSnap, err = a.Store.FindByPrefix(ctx, target)
	}
	if err != nil {
		return fmt.Errorf("diff: %w", err)
	}

	// Find the snapshot immediately before toSnap.
	fromSnap, err := a.Store.PreviousOf(ctx, toSnap.Key)
	if err != nil {
		fmt.Printf("First snapshot — no previous to compare.\n")
		fmt.Printf("Files in %s: (run membox log)\n", toSnap.Key)
		return nil
	}

	fmt.Printf("Diff: %s → %s\n", color.Dim(fromSnap.Key), color.Yellow(toSnap.Key))
	fmt.Println("─────────────────────────────────────────────────────────")

	if statOnly {
		stat, err := a.Differ.Stat(ctx, fromSnap.Key, toSnap.Key)
		if err != nil {
			return err
		}
		fmt.Printf("%s  %s  %s\n",
			color.Green(fmt.Sprintf("+ %d added", stat.Added)),
			color.Red(fmt.Sprintf("- %d deleted", stat.Deleted)),
			color.Yellow(fmt.Sprintf("~ %d modified", stat.Modified)),
		)
		return nil
	}

	out := make(chan diffpkg.Entry, 256)
	errCh := make(chan error, 1)
	go func() {
		errCh <- a.Differ.Diff(ctx, fromSnap.Key, toSnap.Key, out)
	}()

	var stat diffpkg.Stat
	var added, deleted, modified []string

	for entry := range out {
		switch entry.Change {
		case diffpkg.Added:
			stat.Added++
			added = append(added, entry.Path)
		case diffpkg.Deleted:
			stat.Deleted++
			deleted = append(deleted, entry.Path)
		case diffpkg.Modified:
			stat.Modified++
			modified = append(modified, entry.Path)
		}
	}

	if err := <-errCh; err != nil {
		return err
	}

	fmt.Printf("%s\n%s\n%s\n\n",
		color.Green(fmt.Sprintf("  + %d added", stat.Added)),
		color.Red(fmt.Sprintf("  - %d deleted", stat.Deleted)),
		color.Yellow(fmt.Sprintf("  ~ %d modified", stat.Modified)),
	)

	if nameOnly || nameStatus {
		const limit = 50
		printFileList("Added", "A", added, nameStatus, limit, color.Green)
		printFileList("Deleted", "D", deleted, nameStatus, limit, color.Red)
		printFileList("Modified", "M", modified, nameStatus, limit, color.Yellow)
	}

	return nil
}

func printFileList(label, code string, paths []string, withCode bool, limit int, style func(string) string) {
	if len(paths) == 0 {
		return
	}
	fmt.Printf("%s:\n", label)
	for i, p := range paths {
		if i >= limit {
			fmt.Printf("  … and %d more\n", len(paths)-limit)
			break
		}
		if withCode {
			fmt.Printf("  %s %s\n", style(code), p)
		} else {
			fmt.Printf("  %s\n", p)
		}
	}
	fmt.Println()
}
