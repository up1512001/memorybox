package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/up1512001/memorybox/internal/app"
	"github.com/up1512001/memorybox/internal/color"
	"github.com/up1512001/memorybox/internal/restore"
)

func newRestoreCmd(a *app.App) *cobra.Command {
	var (
		snapshot string
		to       string
		dryRun   bool
	)

	cmd := &cobra.Command{
		Use:     "restore <pattern>",
		Aliases: []string{"find"},
		Short:   "Find and recover files from archives (like git checkout)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRestore(cmd.Context(), a, args[0], snapshot, to, dryRun)
		},
	}

	cmd.Flags().StringVar(&snapshot, "snapshot", "", "search only a specific snapshot key or prefix")
	cmd.Flags().StringVar(&to, "to", "", "destination directory for restored files")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "list matches without copying")

	return cmd
}

func runRestore(ctx context.Context, a *app.App, pattern, snapshotKey, destDir string, dryRun bool) error {
	if err := a.CheckDrive(0); err != nil {
		return err
	}

	fmt.Printf("Searching for: %s\n", color.Yellow(pattern))
	fmt.Println("─────────────────────────────────────────────────────────")

	out := make(chan restore.Match, 128)
	errCh := make(chan error, 1)

	go func() {
		errCh <- a.Scanner.Find(ctx, restore.FindOpts{
			Pattern:     pattern,
			SnapshotKey: snapshotKey,
		}, out)
	}()

	var matches []restore.Match
	for m := range out {
		matches = append(matches, m)
		if m.SnapshotKey == "latest" {
			fmt.Printf("  %s %s (%s)\n",
				color.Green("[latest]"), m.Path, humanBytes(m.Size))
		} else {
			fmt.Printf("  %s %s (%s)\n",
				color.Yellow(fmt.Sprintf("[%s]", m.SnapshotKey)),
				m.Path, humanBytes(m.Size))
		}
	}

	if err := <-errCh; err != nil {
		return err
	}

	if len(matches) == 0 {
		fmt.Println("  No matches found.")
		return nil
	}

	fmt.Println()

	if destDir == "" || dryRun {
		fmt.Println("To restore, specify --to <directory>:")
		fmt.Printf("  membox restore %q --to ~/Desktop/restored\n", pattern)
		return nil
	}

	// Copy all matches to destination.
	copied := 0
	for _, m := range matches {
		if err := a.Scanner.Copy(m, restore.CopyOpts{
			Destination: destDir,
			DryRun:      dryRun,
		}); err != nil {
			a.Printer.Warn(fmt.Sprintf("copy %s: %v", m.Path, err))
		} else {
			copied++
		}
	}

	a.Printer.Success(fmt.Sprintf("Restored %d files to %s", copied, destDir))
	return nil
}
