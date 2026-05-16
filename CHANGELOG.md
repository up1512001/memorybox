# Changelog

All notable changes to Memory Box are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [Unreleased]

---

## [0.0.1] — 2026-05-16

Initial release. Core backup engine, CLI, and distribution setup.

### Added

**CLI commands**
- `membox` / `membox backup` — incremental rsync snapshot across 9 configurable sections
- `membox log` — snapshot history with `--oneline`, `--all`, `-n` flags
- `membox diff [snapshot]` — O(n) sort-merge diff with `--stat`, `--name-only`, `--name-status`
- `membox restore <pattern>` — search across all archives with `--to`, `--snapshot`, `--dry-run`
- `membox status` — drive health, free space, days-since-last-backup
- `membox prune` — age + keep-N retention policy with `--dry-run`, `--force`

**Manifest system**
- Tab-delimited format: `<mode>\t<size>\t<mtime>\t<path>` — handles filenames with spaces correctly
- Streaming `Reader`/`Writer` via `bufio` — processes 1M+ entries without loading into memory
- O(n) sort-merge `Merge` — diff requires O(1) memory regardless of manifest size
- Parallel file walker using `filepath.WalkDir` + bounded worker pool (replaces `find -exec stat {} \;`)

**Architecture**
- `internal/manifest` — streaming I/O, merge, parallel walk
- `internal/rsync` — `Runner` interface + subprocess `Exec`, itemize-changes parser
- `internal/config` — Viper merge: flags > `MEMBOX_*` env > YAML file > defaults
- `internal/snapshot` — filesystem store, RFC3339 lexicographic keys
- `internal/scheduler` — semaphore-bounded worker pool (default N=2 concurrent sections)
- `internal/diff` — `StreamDiffer` over manifest merge, Added/Deleted/Modified
- `internal/restore` — `Scanner` walks archive dirs + current backup
- `internal/prune` — age + keep-N policy
- `internal/drive` — `Statfs` probe, disk space pre-check (darwin/linux build tags)
- `internal/color` — TTY detection, `NO_COLOR` env support, named `Style` funcs
- `internal/output` — `Printer` interface, TTY and Quiet implementations

**Distribution**
- GoReleaser: darwin/amd64 + darwin/arm64 binaries, Homebrew tap auto-update
- GitHub Actions release workflow on `v*.*.*` tags
- `.devcontainer/devcontainer.json` for GitHub Codespaces (Go 1.22, Linux build)

[0.0.1]: https://github.com/up1512001/memorybox/releases/tag/v0.0.1
[Unreleased]: https://github.com/up1512001/memorybox/compare/v0.0.1...HEAD
