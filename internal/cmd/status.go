package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/up1512001/memorybox/internal/app"
	"github.com/up1512001/memorybox/internal/color"
	"github.com/up1512001/memorybox/internal/snapshot"
)

func newStatusCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show backup health and drive space (like git status)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runStatus(cmd.Context(), a)
		},
	}
	return cmd
}

func runStatus(ctx context.Context, a *app.App) error {
	if err := a.CheckDrive(0); err != nil {
		return err
	}

	fmt.Println("Backup Status")
	fmt.Println("─────────────────────────────────────────────────────────")

	// Drive info.
	info, err := a.Drive.Probe(ctx, a.Cfg.Drive.MountPath)
	if err == nil {
		pct := int(float64(info.UsedBytes) / float64(info.TotalBytes) * 100)
		bar := progressBar(pct, 30)
		fmt.Printf("Drive:     %s  [%s] %d%% used\n",
			a.Cfg.Drive.MountPath, bar, pct)
		fmt.Printf("           %s free of %s total\n",
			color.Green(humanBytes(info.FreeBytes)),
			humanBytes(info.TotalBytes),
		)
		if info.FreeBytes < 5<<30 { // < 5GB
			fmt.Printf("           %s\n", color.Red("⚠ low disk space"))
		}
	}

	fmt.Println()

	// Last backup.
	last, err := a.Store.Latest(ctx)
	if err == snapshot.ErrNoSnapshots {
		fmt.Println("Last backup:  never")
		fmt.Println()
		fmt.Printf("Run %s to create your first snapshot.\n", color.Cyan("membox"))
		return nil
	}
	if err != nil {
		return err
	}

	daysAgo := int(time.Since(last.CreatedAt).Hours() / 24)
	label := last.Label
	if r, closeR, err := a.Store.ManifestReader(last.Key); err == nil {
		label = r.Header.Message
		closeR()
	}

	switch {
	case daysAgo == 0:
		fmt.Printf("Last backup:  %s — today at %s — %q\n",
			color.Yellow(last.Key),
			last.CreatedAt.Local().Format("15:04"),
			label)
	case daysAgo == 1:
		fmt.Printf("Last backup:  %s — yesterday — %q\n", color.Yellow(last.Key), label)
	case daysAgo <= 7:
		fmt.Printf("Last backup:  %s — %d days ago — %q\n",
			color.Yellow(last.Key), daysAgo, label)
	default:
		fmt.Printf("Last backup:  %s — %s — %q\n",
			color.Yellow(last.Key),
			color.Red(fmt.Sprintf("%d days ago — overdue!", daysAgo)),
			label)
	}

	// Snapshot count and archive usage.
	all, _ := a.Store.List(ctx)
	fmt.Printf("Snapshots:    %d total\n", len(all))
	fmt.Println()
	fmt.Println("Run " + color.Cyan("membox log") + " to see full history.")
	return nil
}

func progressBar(pct, width int) string {
	filled := pct * width / 100
	bar := ""
	for i := 0; i < width; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	return bar
}
