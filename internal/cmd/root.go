// Package cmd contains all cobra subcommand definitions.
package cmd

import (
	"github.com/spf13/cobra"
	"github.com/up1512001/memorybox/internal/app"
)

// persistent flag names shared across subcommands.
const (
	flagConfig   = "config"
	flagDrive    = "drive"
	flagNoColor  = "no-color"
	flagQuiet    = "quiet"
	flagVerbose  = "verbose"
	flagDryRun   = "dry-run"
	flagParallel = "parallel"
)

// NewRootCmd builds the root cobra command and attaches all subcommands.
func NewRootCmd(a *app.App) *cobra.Command {
	var cfgFile string
	var noColor bool

	root := &cobra.Command{
		Use:   "membox",
		Short: "Memory Box — git-like Mac backup powered by rsync",
		Long: `Memory Box creates incremental snapshots of your Mac to an external drive.
Deleted and overwritten files are archived — nothing is ever lost.

Usage:
  membox init                    first-time setup (pick drive, configure sections)
  membox                         run a backup (like git commit)
  membox -m "before big refactor" backup with a label
  membox log                     show snapshot history
  membox diff [snapshot]         what changed between snapshots
  membox restore <pattern>       find and recover files
  membox status                  drive health and space
  membox prune                   clean old archives`,
		Version: a.Build.Version + " (" + a.Build.Commit + ")",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// Override drive path if --drive flag was set.
			driveFlag := cmd.Root().PersistentFlags().Lookup(flagDrive)
			if driveFlag != nil && driveFlag.Changed {
				a.V.Set("drive.mountPath", driveFlag.Value.String())
			}
			return a.Init(cfgFile, noColor)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Persistent flags available to all subcommands.
	pf := root.PersistentFlags()
	pf.StringVar(&cfgFile, flagConfig, "", "config file (default: ~/.config/memorybox/config.yaml)")
	pf.String(flagDrive, "", "override drive mount path (default: /Volumes/X10 Pro)")
	pf.BoolVar(&noColor, flagNoColor, false, "disable color output")
	pf.BoolP(flagQuiet, "q", false, "suppress output (for cron/automation)")
	pf.BoolP(flagVerbose, "v", false, "show rsync progress per file")

	// Bind persistent flags to viper.
	a.V.BindPFlag("ui.quiet", pf.Lookup(flagQuiet))
	a.V.BindPFlag("ui.verbose", pf.Lookup(flagVerbose))

	// Attach subcommands.
	root.AddCommand(
		newBackupCmd(a),
		newInitCmd(a),
		newLogCmd(a),
		newDiffCmd(a),
		newRestoreCmd(a),
		newStatusCmd(a),
		newPruneCmd(a),
	)

	return root
}
