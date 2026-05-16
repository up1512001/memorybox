package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/up1512001/memorybox/internal/app"
	"github.com/up1512001/memorybox/internal/color"
)

func newLogCmd(a *app.App) *cobra.Command {
	var (
		count   int
		all     bool
		oneLine bool
	)

	cmd := &cobra.Command{
		Use:   "log",
		Short: "Show snapshot history (like git log)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runLog(cmd.Context(), a, count, all, oneLine)
		},
	}

	cmd.Flags().IntVarP(&count, "count", "n", 10, "number of snapshots to show")
	cmd.Flags().BoolVar(&all, "all", false, "show all snapshots")
	cmd.Flags().BoolVar(&oneLine, "oneline", false, "compact single-line per snapshot")

	return cmd
}

func runLog(ctx context.Context, a *app.App, count int, all, oneLine bool) error {
	if err := a.CheckDrive(0); err != nil {
		return err
	}

	snaps, err := a.Store.List(ctx)
	if err != nil {
		return fmt.Errorf("log: %w", err)
	}

	if len(snaps) == 0 {
		fmt.Println("No backups yet. Run: membox")
		return nil
	}

	if !all && count > 0 && len(snaps) > count {
		snaps = snaps[:count]
	}

	fmt.Printf("Backup History (%d snapshots)\n", len(snaps))
	fmt.Println("─────────────────────────────────────────────────────────")

	for _, snap := range snaps {
		var label, updated, archived, transferred string

		r, closeR, err := a.Store.ManifestReader(snap.Key)
		if err == nil {
			label = r.Header.Message
			updated = fmt.Sprintf("%d", r.Header.Updated)
			archived = fmt.Sprintf("%d", r.Header.Archived)
			transferred = humanBytes(r.Header.Transferred)
			closeR()
		} else {
			label = snap.Label
			updated, archived, transferred = "?", "?", "?"
		}

		archiveInfo := ""
		if stat, err := os.Stat(snap.ArchivePath); err == nil && stat.IsDir() {
			archiveInfo = fmt.Sprintf(" │ has archive")
		}

		if oneLine {
			fmt.Printf("%s  %s updated, %s archived, %s — %s\n",
				color.Yellow(snap.Key), updated, archived, transferred, label)
		} else {
			fmt.Printf("%s  %s updated, %s archived, %s sent%s\n",
				color.Yellow(snap.Key), updated, archived, transferred, archiveInfo)
			fmt.Printf("  %s\n\n", label)
		}
	}
	return nil
}
