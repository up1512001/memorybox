# Roadmap

Priorities drawn from community research: MacRumors forums, Hacker News, Reddit
(r/MacOS, r/mac, r/sysadmin), open-source project READMEs (tmignore, asimov,
shallow-backup, yarsync), and reviews of competing tools (Arq, CCC, restic,
Kopia). Items ordered by community demand.

---

## v0.1 — Trust & reliability

The #1 complaint across every forum: backups look fine until restore fails.

### `membox verify` — integrity check
Walk `backup-current/` and compare checksums against the latest manifest.
Flag files whose size or mtime diverged without a snapshot being taken
(interrupted rsync, disk corruption). Exit non-zero so it can be scripted.

> "Time Machine works fine until it doesn't. And it won't tell you that a backup
> is broken until you try to restore from it." — Hacker News

### Pinned rsync detection
Apple replaced rsync with openrsync in Sequoia and broke `--backup-dir` in 15.4.
Detect at startup if the system rsync is too old and suggest `brew install rsync`.

> "Apple's command-line tools are unreliable and shouldn't be trusted for
> critical tasks." — mjtsai.com

### macOS failure notifications
Native macOS notification via `osascript` on backup failure. Non-zero exit codes
on all commands so cron/launchd scripts can react.

---

## v0.2 — Developer experience

Most-requested features from the developer segment.

### gitignore-aware exclusions
Scan source trees for `.gitignore` files and auto-exclude matched patterns
(node_modules, vendor, dist, build, etc.). Replaces the tools tmignore and
asimov, both built specifically because no backup tool handles this.

> "You can exclude specific node_modules folders, but you can't have a global
> exclusion rule." — MacRumors developer thread

Opt-in per section via `gitignoreExcludes: true` in config.

### Pre/post backup hooks
Run a shell command before or after each section or the full snapshot.

```yaml
sections:
  dev:
    hooks:
      pre: "pg_dump mydb > ~/Developer/db-snapshot.sql"
      post: "curl -s $SLACK_WEBHOOK -d '{\"text\":\"backup done\"}'"
```

Covers the top use cases: database dump before backup, Slack/Discord notification
after, NAS wake/sleep.

### `membox schedule` — LaunchAgent setup
Write a `com.memorybox.backup.plist` to `~/Library/LaunchAgents` and load it.
Scheduled backups without manually wiring launchd. Includes rsync `--bwlimit`
and `nice` options so backup doesn't saturate the connection or peg CPU.

> "To use restic I'd have to wrap it in my own backup script and run it via
> launchd — implementing notifications, bandwidth throttling and CPU limits
> requires custom scripting." — dzombak.com

---

## v0.3 — Storage & speed

### BoltDB index for instant restore
`membox restore` currently walks all archive dirs linearly. Index filenames +
snapshot keys in BoltDB (`go.etcd.io/bbolt`, CGO-free). Makes restore instant
regardless of archive size.

### `membox watch` — backup on drive connect
Use FSEvents to detect when the configured SSD is plugged in and trigger a
backup automatically. Top-requested "set and forget" feature across all forums.

---

## v0.4 — Security

### Manifest encryption (`age`)
Optionally encrypt `.manifest` files using `filippo.io/age`. Manifests reveal
filenames, sizes, and mtimes — worth protecting for sensitive sections
(`.ssh`, `.gnupg`, financial docs). File data is not stored by membox (rsync
writes directly to drive).

> "Even if you trust a third party, if it holds the private encryption keys to
> your data then it can get hacked." — privacy forum thread

---

## v0.5 — Remote destinations

### rsync-over-SSH to NAS
Time Machine over SMB to Synology/QNAP is extensively documented as broken —
periodic corruption, CPU pegging, requires full restart after failure. rsync-over-SSH
is the proven alternative.

```yaml
drive:
  mountPath: "user@nas.local:/volume1/membox-backup"
  protocol: ssh
```

Requires abstracting local-path assumptions in `snapshot.Store` and `drive.Prober`.

> "I could never get TM backup to Synology to work without corruption after a
> month or two." — SNBForums

---

## Long-term / exploratory

- **APFS snapshot source** — `tmutil snapshot` before rsync for consistent point-in-time capture; avoids "file changed during backup" errors in Photos and large repos
- **Block-level delta** — chunked hashing for large files (databases, VM images); significant architecture change
- **Multi-machine consolidation** — multiple Macs to one NAS under separate prefixes, `membox log --all` aggregates
- **`membox serve`** — local read-only web dashboard for history, diff, and restore preview; reuses existing packages

---

## Won't do

| Item | Reason |
|------|--------|
| Windows support | `/Volumes/`, FSEvents, and rsync semantics are macOS/Linux only |
| Cloud-only storage | Designed around local + NAS drives |
| GUI app | CLI UX is the product |
| Bootable clone | Deprecated by Apple in macOS 15.2; CCC and SuperDuper are already broken |
| Proprietary format | Everything is plain rsync output + tab-delimited manifests |
