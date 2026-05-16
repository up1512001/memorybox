# Contributing

## Setup

```bash
git clone https://github.com/up1512001/memorybox
cd memorybox
go mod tidy
go build -o ./bin/membox ./cmd/memorybox
```

Requirements: Go 1.22+, `rsync` (ships with macOS).

## Testing

```bash
go test ./...                    # all tests
go test -race ./...              # with race detector
go test -short ./...             # skip slow tests

# Benchmarks
go test -bench=. -benchmem -run='^$' ./internal/manifest/...
go test -bench=BenchmarkMerge1M -benchtime=3x -run='^$' ./internal/manifest/
```

## Project structure

```
cmd/memorybox/     entry point (ldflags: version, commit, date)
internal/
  manifest/        streaming I/O, O(n) merge, parallel walk  ← start here
  rsync/           Runner interface + subprocess wrapper
  config/          Viper schema, defaults, loader
  snapshot/        filesystem store
  scheduler/       semaphore worker pool
  diff/            StreamDiffer (uses manifest.Merge)
  restore/         archive scanner
  prune/           retention policy
  drive/           Statfs probe (darwin/linux split)
  color/           TTY detection, NO_COLOR
  output/          Printer interface
  app/             wires subsystems, PersistentPreRunE
  cmd/             cobra subcommands
```

See [`AGENTS.md`](AGENTS.md) for deeper architecture notes.

## Key constraints

- **Manifests must stay streamable.** Never `ReadAll` a manifest file. Use `manifest.Reader.Next()`.
- **Diff is O(n) sort-merge.** Both manifests must remain sorted by path at write time. Don't break this.
- **No CGO.** `CGO_ENABLED=0` for single static binary. Don't introduce C dependencies.
- **Darwin + Linux build.** OS-specific code goes in `_darwin.go` / `_linux.go` files with `//go:build` tags.
- **All paths through `Printer`.** No `fmt.Printf` in command handlers — use `a.Printer.*`.

## Adding a new section

1. Add an entry to `DefaultSections()` in `internal/config/defaults.go`
2. Handle the section name in `enabledSections()` in `internal/cmd/backup.go`
3. Add it to the section order slice in `backup.go`

## Releasing

```bash
git tag v0.x.y
git push origin main --tags
```

GitHub Actions runs GoReleaser, builds darwin/amd64 + arm64, publishes to Homebrew tap.

## Commit style

```
type: short description (imperative, lowercase)

Optional body explaining why, not what.
```

Types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`.
