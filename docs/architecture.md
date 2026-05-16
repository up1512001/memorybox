# Architecture

## Overview

Memory Box is a Mac-only CLI backup tool that wraps `rsync`. It layers a
git-like interface (snapshot history, diff, restore, prune) on top of rsync's
incremental transfer engine and a simple file-based manifest store.

```
membox (CLI)
│
├── internal/cmd/         cobra subcommands
│   ├── backup.go         orchestrates rsync runs, builds manifests
│   ├── init.go           interactive setup wizard
│   ├── log.go            reads manifest headers from store
│   ├── diff.go           drives StreamDiffer
│   ├── restore.go        drives Scanner
│   ├── status.go         drives drive.Prober
│   └── prune.go          drives Pruner
│
├── internal/app/         wires all subsystems (App struct)
├── internal/config/      schema, loader (Viper), defaults, wizard writer
│
├── internal/manifest/    ← foundation of everything
│   ├── entry.go          tab-delimited wire format
│   ├── reader.go         streaming line-by-line reader
│   ├── writer.go         buffered writer, maintains sorted order
│   ├── merge.go          O(n) sort-merge of two sorted readers
│   └── walker.go         parallel WalkDir → sorted []Entry
│
├── internal/rsync/       subprocess wrapper
│   ├── runner.go         Runner interface + Exec impl
│   ├── args.go           BuildArgs — no shell involved
│   └── parser.go         itemize-changes + stats parser
│
├── internal/snapshot/    filesystem store (manifest + archive dirs)
├── internal/scheduler/   semaphore worker pool for parallel sections
├── internal/diff/        StreamDiffer over manifest.Merge
├── internal/restore/     archive scanner
├── internal/prune/       age + keep-N retention policy
├── internal/drive/       Statfs probe (darwin/linux build tags)
├── internal/color/       TTY detection, NO_COLOR, named Style funcs
└── internal/output/      Printer interface (TTY and Quiet impls)
```

## Data flow — backup

```
membox backup
    │
    ├─ scheduler.Run() ─────── parallel rsync per section
    │      │
    │      └─ rsync.Exec.Run() → subprocess
    │             ├── --backup-dir=<archive>/<key>/   # moves deleted files
    │             └── itemize-changes output → rsync.Line stream
    │
    └─ manifest.Walk(backup-current/) → sorted []Entry
           │
           └─ manifest.Writer → backup-manifests/<key>.manifest
```

## Data flow — diff

```
membox diff <key>
    │
    ├─ snapshot.Store.FindByPrefix(key)   → Snapshot A, Snapshot B
    ├─ manifest.Reader (A) ─── both sorted by path
    ├─ manifest.Reader (B) ─┘
    │
    └─ manifest.Merge() ──── O(n) sort-merge, emits MergeEvent
           │
           └─ diff.Differ consumes events → Added / Deleted / Modified
```

## Manifest format

Every snapshot writes one `.manifest` file:

```
# message: Weekly backup
# snapshot: 2026-05-16T14-30-00Z
# updated: 120
# archived: 5
# transferred: 1234567890
33188	1024	1716000000	Documents/report.pdf
33188	5678	1716000001	iCloud Drive/My Project/Design Doc.pages
```

- Header lines start with `#`
- Data lines: `<mode>\t<size>\t<mtime>\t<path>` — path is last so spaces in filenames are safe
- Lines are written in ascending lexicographic path order (required by `manifest.Merge`)

## On-disk layout

```
/Volumes/X10 Pro/                 ← drive.mountPath
├── backup-current/               ← live mirror (rsync destination)
│   ├── Pictures/
│   ├── Documents/
│   └── ...
├── backup-archive/
│   ├── 2026-05-16T14-30-00Z/     ← files deleted/overwritten this snapshot
│   └── 2026-05-09T10-15-00Z/
├── backup-manifests/
│   ├── 2026-05-16T14-30-00Z.manifest
│   └── 2026-05-09T10-15-00Z.manifest
└── backup-logs/
    ├── 2026-05-16T14-30-00Z.log
    └── history.csv
```

## Snapshot keys

RFC3339 UTC timestamps with colons replaced by dashes:
`2026-05-16T14-30-00Z`

Lexicographic sort == chronological sort. No date parsing needed for ordering.

## Configuration priority

```
CLI flags  >  MEMBOX_* env vars  >  ~/.config/memorybox/config.yaml  >  built-in defaults
```

Run `membox init` to generate the config file interactively.

## Key design decisions

| Decision | Reason |
|----------|--------|
| Streaming manifest I/O (never ReadAll) | Handles 1M+ files with O(1) memory |
| O(n) sort-merge diff | No map lookups; 1M entries diff in ~1.5s |
| Tab-delimited manifest (path last) | Spaces in filenames safe without quoting |
| No CGO | Single static binary, works in Codespaces |
| Build tags for OS split | Clean darwin/linux without `!linux` hacks |
| All output via Printer interface | --quiet mode works throughout |
| rsync --backup-dir | Deleted files moved atomically, never lost |
| RFC3339 keys, lexicographic sort | No date parsing needed for ordering ops |

## Concurrency model

`internal/scheduler` runs sections in parallel via a semaphore channel
(`make(chan struct{}, maxWorkers)`). Default is 2 concurrent sections — USB
I/O is the bottleneck, not CPU, so more workers rarely helps.

`manifest.Walk` uses `filepath.WalkDir` + a bounded worker pool
(`runtime.NumCPU()` goroutines) for parallel `os.Lstat` calls. This replaces
the old `find -exec stat {} \;` approach (one subprocess per file).
