# Roadmap

Priorities drawn from community research: MacRumors forums, Hacker News, Reddit
(r/MacOS, r/mac, r/sysadmin), open-source project READMEs (tmignore, asimov,
shallow-backup, yarsync), and reviews of competing tools (Arq, CCC, restic,
Kopia). Items ordered by community demand.

---

## ~~v0.1 — Trust, reliability & Linux support~~ ✅ shipped in v0.1.0

### ~~Ubuntu / Linux support~~ ✅
Full Linux support — not just a build stub.

- `drive_linux.go` — real `Statfs` via `syscall.Statfs_t`, identical to macOS
- `volumes_linux.go` — scans `/media/$USER`, `/run/media/$USER` (Fedora), `/mnt`
  for actual mount points using device-ID comparison (not guessing)
- Linux binaries ship in releases (`linux/amd64`, `linux/arm64`)
- Install on Ubuntu: `brew install up1512001/tap/membox` (Linuxbrew) or download tarball

### ~~`membox verify` — integrity check~~ ✅
Walk `backup-current/` and compare checksums against the latest manifest.
Flag files whose size or mtime diverged without a snapshot being taken
(interrupted rsync, disk corruption). Exit non-zero so it can be scripted.

> "Time Machine works fine until it doesn't. And it won't tell you that a backup
> is broken until you try to restore from it." — Hacker News

### ~~Pinned rsync detection~~ ✅
Apple replaced rsync with openrsync in Sequoia and broke `--backup-dir` in 15.4.
Detect at startup if the system rsync is too old and suggest `brew install rsync`.

### ~~macOS / Linux failure notifications~~ ✅
Native macOS notification via `osascript` on backup success/failure; Linux
`notify-send` with silent fallback. Non-zero exit codes on all commands so
cron/launchd scripts can react.

---

## ~~v0.2 — Developer experience~~ ✅ shipped in v0.2.0

### ~~gitignore-aware exclusions~~ ✅
Scan source trees for `.gitignore` files and auto-exclude matched patterns
(node_modules, vendor, dist, build, etc.). Opt-in via `gitignoreAware: true`
on a section.

> "You can exclude specific node_modules folders, but you can't have a global
> exclusion rule." — MacRumors developer thread

### ~~Pre/post backup hooks~~ ✅

```yaml
sections:
  dev:
    hooks:
      pre: "pg_dump mydb > ~/Developer/db-snapshot.sql"
      post: "curl -s $SLACK_WEBHOOK -d '{\"text\":\"backup done\"}'"
```

### ~~`membox schedule` — LaunchAgent / systemd setup~~ ✅
- **macOS**: write `com.memorybox.backup.plist` to `~/Library/LaunchAgents`
- **Linux**: write a `membox-backup.service` + timer to `~/.config/systemd/user/`
- Includes `--bwlimit` and `nice` options

---

## ~~v0.3 — Storage & speed~~ ✅ shipped in v0.3.0

### ~~BoltDB index for instant restore~~ ✅
`membox restore` currently walks all archive dirs linearly. Index filenames +
snapshot keys in BoltDB (`go.etcd.io/bbolt`, CGO-free). Instant regardless of
archive size.

### ~~`membox watch` — backup on storage connect~~ ✅
- **macOS**: FSEvents on `/Volumes`
- **Linux**: inotify on `/media/$USER` via `fsnotify`

---

## v0.4 — Cloud sync

Back up directly to cloud storage — no physical drive required. Uses rclone as
the transport layer so any of rclone's 40+ providers work with zero extra code.

### Architecture
`rsync.Runner` is already an interface. Add `RcloneRunner` that wraps `rclone sync`
with the same `--itemize-changes` output parsing. The manifest, snapshot, diff,
restore, and prune packages stay unchanged — they operate on metadata, not transport.

```yaml
drive:
  backend: rclone             # "local" (default) or "rclone"
  rclonePath: "r2:my-bucket/membox"   # any rclone remote:path
```

### Supported backends (via rclone)

| Provider | Config example |
|----------|---------------|
| Cloudflare R2 | `r2:bucket-name/membox` |
| AWS S3 | `s3:bucket-name/membox` |
| Backblaze B2 | `b2:bucket-name/membox` |
| Google Cloud Storage | `gcs:bucket-name/membox` |
| Any S3-compatible | custom rclone remote |

### Archive strategy for cloud
`rsync --backup-dir` doesn't work remotely. Replace with a pre-backup step that
copies changed files to a dated archive prefix before syncing:
`r2:bucket/membox/archive/2026-05-16T14-30-00Z/` — same layout as local.

### Manifest sync
After each backup, push the `.manifest` file to the cloud path. On restore,
pull the latest manifest to a local cache dir (`~/.cache/memorybox/manifests/`).

### Initial implementation scope
- `membox backup` with `rclone` backend
- `membox log` from cached manifests
- `membox restore` from cloud (rclone copy individual file)
- `membox diff` from cached manifests

---

## v0.5 — Security

### Manifest encryption (`age`)
Optionally encrypt `.manifest` files using `filippo.io/age`. Manifests reveal
filenames, sizes, and mtimes — worth protecting for sensitive sections
(`.ssh`, `.gnupg`, financial docs).

---

## v0.6 — Remote NAS (rsync-over-SSH)

For users who want the local-drive experience but over the network.

```yaml
drive:
  backend: ssh
  sshPath: "user@nas.local:/volume1/membox-backup"
```

Time Machine over SMB to Synology/QNAP is extensively documented as broken.
rsync-over-SSH is the reliable alternative.

---

## Long-term / exploratory

- **APFS snapshot source** — `tmutil snapshot` before rsync for consistent point-in-time capture
- **Block-level delta** — chunked hashing for large files (databases, VM images)
- **Multi-machine consolidation** — multiple machines to one NAS/bucket, `membox log --all`
- **`membox serve`** — local read-only web dashboard for history, diff, restore preview

---

## Won't do

| Item | Reason |
|------|--------|
| Windows support | FSEvents, `/Volumes/`, rsync semantics are macOS/Linux only |
| GUI app | CLI UX is the product |
| Bootable clone | Apple deprecated in macOS 15.2; CCC and SuperDuper are already broken |
| Proprietary format | Everything is plain rsync output + tab-delimited manifests |
