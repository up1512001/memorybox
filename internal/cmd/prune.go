package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/up1512001/memorybox/internal/app"
	"github.com/up1512001/memorybox/internal/color"
	prunepkg "github.com/up1512001/memorybox/internal/prune"
)

func newPruneCmd(a *app.App) *cobra.Command {
	var (
		days   int
		keep   int
		dryRun bool
		force  bool
	)

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Delete old archives to reclaim disk space",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPrune(cmd.Context(), a, days, keep, dryRun, force)
		},
	}

	cmd.Flags().IntVar(&days, "days", 0, "delete archives older than N days (default from config)")
	cmd.Flags().IntVar(&keep, "keep", 0, "always keep last N snapshots (default from config)")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "show what would be deleted")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")

	return cmd
}

func runPrune(ctx context.Context, a *app.App, days, keep int, dryRun, force bool) error {
	if err := a.CheckDrive(0); err != nil {
		return err
	}

	// Fall back to config defaults.
	if days == 0 {
		days = a.Cfg.Prune.DefaultDays
	}
	if keep == 0 {
		keep = a.Cfg.Prune.DefaultKeep
	}

	opts := prunepkg.Opts{
		OlderThanDays: days,
		KeepLast:      keep,
		DryRun:        dryRun,
	}

	candidates, err := a.Pruner.Candidates(ctx, opts)
	if err != nil {
		return fmt.Errorf("prune: %w", err)
	}

	if len(candidates) == 0 {
		fmt.Println("Nothing to prune.")
		return nil
	}

	fmt.Printf("Pruning archives older than %d days (keeping last %d)\n", days, keep)
	fmt.Println("─────────────────────────────────────────────────────────")

	for _, snap := range candidates {
		fmt.Printf("  %s %s — %q\n",
			color.Red("remove"),
			snap.Key, snap.Label)
	}
	fmt.Println()

	if dryRun {
		fmt.Printf("Dry-run: %d snapshots would be removed.\n", len(candidates))
		return nil
	}

	if !force {
		fmt.Printf("Remove %d snapshots? [y/N]: ", len(candidates))
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		if !strings.EqualFold(strings.TrimSpace(scanner.Text()), "y") {
			fmt.Println("Aborted.")
			return nil
		}
	}

	result, err := a.Pruner.Prune(ctx, opts)
	if err != nil {
		return fmt.Errorf("prune: %w", err)
	}

	a.Printer.Success(fmt.Sprintf("Removed %d snapshots.", len(result.Removed)))
	return nil
}
