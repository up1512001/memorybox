# AGENTS.md — Memory Box Codebase Guide

This file is for AI agents and developers working on the Memory Box codebase.
Read this before making changes.

## What this project is

`membox` is a Mac-only CLI backup tool that wraps `rsync`. It provides git-like
subcommands (`log`, `diff`, `restore`, `prune`) over a filesystem of snapshot
manifests and rsync archives stored on an external SSD.

Binary: `membox` — built from `cmd/memorybox/main.go`.
Module: `github.com/up1512001/memorybox`

## Build & test

```bash
go build -o ./bin/membox ./cmd/memorybox   # build
go test ./...                               # all tests
go test -race ./...                         # race detector
GOOS=linux go build ./...                   # verify Linux compat
go test -bench=BenchmarkMerge1M -benchtime=3x -run='^$' ./internal/manifest/
```

## Package map

| Package | Role | Key types |
|---------|------|-----------|
| `internal/manifest` | Foundation for all snapshot I/O | `Entry`, `Reader`, `Writer`, `Merge`, `Walk` |
| `internal/rsync` | Subprocess wrapper | `Runner` (interface), `Exec` (impl), `BuildArgs`, `Stats` |
| `internal/config` | Config loading | `Config`, `SectionConfig`, `Load()` |
| `internal/snapshot` | Snapshot lifecycle | `Store`, `Snapshot` |
| `internal/scheduler` | Parallel section runner | `Scheduler`, `Job`, `Result` |
| `internal/diff` | Snapshot comparison | `Differ`, `Entry`, `Stat`, `ChangeType` |
| `internal/restore` | File search & recovery | `Scanner`, `Match`, `FindOpts` |
| `internal/prune` | Retention policy | `Pruner`, `Opts`, `Result` |
| `internal/drive` | Drive health | `Prober`, `Info` |
| `internal/color` | Terminal styling | `Style`, `Init()` |
| `internal/output` | Output abstraction | `Printer` (interface), `TTY`, `Quiet` |
| `internal/app` | Subsystem wiring | `App`, `Init()`, `CheckDrive()` |
| `internal/cmd` | Cobra commands | one file per subcommand |

## Critical invariants — do not break these

### 1. Manifests must stay streamable

Never load a full manifest into memory. Always use `manifest.Reader.Next()`:

```go
// CORRECT
r, close, _ := store.ManifestReader(key)
defer close()
for {
    e, err := r.Next()
    if err == io.EOF { break }
    // process e
}

// WRONG — do not do this
data, _ := os.ReadFile(manifestPath)
lines := strings.Split(string(data), "\n") // breaks for 1M+ entries
```

### 2. Manifest entries must stay sorted by path

`manifest.Merge` (and therefore `diff.Differ`) requires both readers to be in
ascending lexicographic path order. `manifest.Walk` sorts before returning.
`manifest.Writer` writes entries in the order they are given — callers must
feed them sorted.

### 3. Diff is O(n) sort-merge — never O(n²)

The merge in `internal/manifest/merge.go` advances two sorted readers in lock-step.
Don't replace it with map lookups or nested loops. For N=1M entries, that's the
difference between 1.5s and minutes.

### 4. No CGO

`CGO_ENABLED=0` throughout. Don't introduce C dependencies. `go.etcd.io/bbolt`
(BoltDB) is available for optional indexing and is CGO-free.

### 5. OS-specific code uses build tags

macOS syscall code lives in `_darwin.go` files with `//go:build darwin`.
Linux stubs live in `_linux.go` files. Always verify:

```bash
GOOS=linux go build ./...   # must pass clean
```

### 6. All output goes through `Printer`

Command handlers never call `fmt.Printf` directly. Use `a.Printer.Info()`,
`a.Printer.Success()`, etc. This keeps `--quiet` mode working correctly.

### 7. Paths with spaces are handled by manifest format

The manifest wire format is tab-delimited: `<mode>\t<size>\t<mtime>\t<path>`.
Path is always the last field so spaces in filenames are safe. **Do not**
use space-delimited formats or `awk '{print $NF}'` patterns on manifest lines.

## Manifest file format

```
# message: Weekly backup
# snapshot: 2026-05-16T14-30-00Z
# updated: 120
# archived: 5
# transferred: 1234567890
33188	1024	1716000000	Documents/report.pdf
33188	5678	1716000001	iCloud Drive/My Project/Design Doc.pages
```

Comment lines (start with `#`) form the header. Data lines are tab-delimited.
`ParseLine()` in `internal/manifest/entry.go` is the canonical parser.

## Snapshot key format

Keys are RFC3339 UTC timestamps with colons replaced by dashes for filesystem
safety: `2026-05-16T14-30-00Z`. Lexicographic sort == chronological sort — this
means no date parsing is needed for ordering operations in `snapshot.Store`.

## Adding a section

1. `internal/config/defaults.go` → add entry to `DefaultSections()`
2. `internal/cmd/backup.go` → add name to the `order` slice in `enabledSections()`
3. Write a test that verifies the section's source path expands correctly

## Adding a subcommand

1. Create `internal/cmd/<name>.go` with a `newXCmd(a *app.App) *cobra.Command` function
2. Register it in `internal/cmd/root.go` inside `root.AddCommand(...)`
3. Use `cmd.Context()` for the context (not `context.Background()`)
4. Write through `a.Printer`, not `fmt.Print*`

## Testing conventions

- Table-driven tests for all pure-logic packages (`manifest`, `diff`, `prune`, `config`)
- Use `t.TempDir()` for filesystem tests — never touch real backup dirs
- Mock `Runner` interface for rsync-dependent tests via `internal/testutil/fakes.go`
- Benchmarks in `_test.go` files with `Benchmark` prefix; run with `-benchmem`
- Race detector: `go test -race ./...` must pass before every commit

## What NOT to do

- Do not `ReadAll` manifest files
- Do not spawn goroutines per file (use worker pools)
- Do not add `fmt.Print*` in command handlers
- Do not use `awk '{print $NF}'` or space-splitting on manifest paths
- Do not add `//go:build !linux` — always write an explicit Linux stub
- Do not skip the Linux build check (`GOOS=linux go build ./...`)
- Do not break the sorted-path invariant in `manifest.Writer`
- Do not use `os.Exit` in library code — only in `cmd/memorybox/main.go`

## Dependency policy

Prefer stdlib. Current external deps and their purpose:

| Module | Purpose |
|--------|---------|
| `github.com/spf13/cobra` | CLI subcommands |
| `github.com/spf13/viper` | Config merge (flags > env > file) |
| `golang.org/x/term` | TTY detection for color |
| `github.com/dustin/go-humanize` | Byte formatting (available, use it) |
| `github.com/stretchr/testify` | Test assertions only |

New dependencies require explicit justification. No CGO.
