# Roadmap

Items are roughly ordered by priority. Nothing here is committed — open an issue
to discuss before building.

## Near-term (v0.1.x)

### `membox watch` — auto-backup on drive connect
Detect when the configured SSD is plugged in and trigger a backup automatically.
Uses `FSEvents` (darwin) or `inotify` (linux) to watch `/Volumes`.

### `membox verify` — manifest integrity check
Walk `backup-current/` and compare against the latest manifest. Flag files
whose size or mtime diverged without a snapshot being taken (e.g. rsync
interrupted mid-run).

### Encrypted manifests
Optionally encrypt `.manifest` files at rest using age encryption
(`filippo.io/age`). Manifest content reveals filenames and sizes but not
file data — worth protecting for sensitive dirs like `.ssh`, `.gnupg`.

### Custom sections in config
Let users define entirely new sections in `config.yaml` with arbitrary
source paths, not just the 9 built-in ones. `membox init` wizard should
support adding custom sections.

---

## Medium-term (v0.2.x)

### BoltDB snapshot index
Replace linear manifest scanning for `restore` with a BoltDB index keyed by
filename. `membox restore` currently does a full walk of all archive dirs;
an index makes it instant. `go.etcd.io/bbolt` is CGO-free and already
listed as an available dependency.

### `membox diff --files` output piped to editor
Pipe `membox diff` output to `$DIFFTOOL` (e.g. `vimdiff`, `delta`) for a
side-by-side view of text file changes between snapshots.

### Progress bar for large transfers
Stream rsync `--progress` output and render a live transfer bar through
the `Printer` interface. Currently only file counts are shown.

### LaunchAgent / launchd integration
`membox install-launchd` writes a `com.memorybox.backup.plist` to
`~/Library/LaunchAgents` and loads it for scheduled backups without cron.

---

## Long-term / exploratory

### Remote destinations (SSH / rclone)
Abstract `rsync.Runner` to support remote destinations via SSH or rclone.
Would require rethinking archive paths and manifest sync.

### Snapshot tags and annotations
Beyond the `-m` message, allow tagging snapshots (`membox tag <key> <name>`)
and filtering log/diff by tag. Stored as extra header lines in the manifest.

### Web UI
A local read-only web dashboard (`membox serve`) for browsing snapshot
history, diffs, and restore previews. Would use the existing manifest
and snapshot packages as a library — no new storage layer needed.

### Multi-machine sync
Consolidate manifests from multiple Macs onto a single NAS. Each machine
writes to its own prefix (`/Volumes/NAS/mac-utsav/`, `/Volumes/NAS/mac-work/`).
`membox log --all-machines` aggregates across them.

---

## Won't do

- **Windows support** — rsync semantics and `/Volumes` detection are macOS/Linux specific.
- **Cloud-only storage** — Memory Box is designed around a local SSD. Cloud sync belongs to a different tool.
- **GUI app** — Out of scope; the CLI UX is the product.
