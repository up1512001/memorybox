package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/up1512001/memorybox/internal/app"
)

func newVerifyCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "verify",
		Short: "Check backup integrity against the latest manifest",
		Long: `Walk backup-current/ and compare each file's size and mtime against
the latest manifest. Flags files that diverged without a snapshot being taken
(interrupted rsync, disk corruption). Exits non-zero if any divergence is found.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runVerify(cmd, a)
		},
	}
}

func runVerify(cmd *cobra.Command, a *app.App) error {
	if err := a.CheckDrive(0); err != nil {
		return err
	}

	snap, err := a.Store.Latest(cmd.Context())
	if err != nil {
		return fmt.Errorf("no snapshots found — run `membox` first")
	}

	r, closeR, err := a.Store.ManifestReader(snap.Key)
	if err != nil {
		return fmt.Errorf("open manifest: %w", err)
	}
	defer closeR()

	a.Printer.Info(fmt.Sprintf("Verifying against snapshot %s …", snap.Key))

	root := a.Cfg.Drive.BackupDir
	var mismatches, missing, checked int

	for {
		entry, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read manifest: %w", err)
		}
		checked++

		diskPath := filepath.Join(root, entry.Path)
		info, err := os.Lstat(diskPath)
		if err != nil {
			if os.IsNotExist(err) {
				a.Printer.Warn(fmt.Sprintf("MISSING  %s", entry.Path))
				missing++
				continue
			}
			a.Printer.Warn(fmt.Sprintf("STAT ERR %s: %v", entry.Path, err))
			mismatches++
			continue
		}

		sizeDiverged := info.Size() != entry.Size
		mtimeDiverged := info.ModTime().UTC().Unix() != entry.MTime.Unix()

		if sizeDiverged || mtimeDiverged {
			reason := divergeReason(sizeDiverged, mtimeDiverged)
			a.Printer.Warn(fmt.Sprintf("DIVERGED %s  (%s)", entry.Path, reason))
			mismatches++
		}
	}

	a.Printer.Info(fmt.Sprintf("Checked %d files — %d missing, %d diverged", checked, missing, mismatches))

	if missing > 0 || mismatches > 0 {
		return fmt.Errorf("integrity check failed: %d issue(s) found", missing+mismatches)
	}

	a.Printer.Success("Backup integrity OK")
	return nil
}

func divergeReason(sizeDiverged, mtimeDiverged bool) string {
	switch {
	case sizeDiverged && mtimeDiverged:
		return "size+mtime"
	case sizeDiverged:
		return "size"
	default:
		return "mtime"
	}
}
