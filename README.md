# Memory Box

Git-like Mac backup powered by rsync. Take snapshots of your Mac to an external SSD. Browse history, diff snapshots, and restore deleted files — all from the terminal.

```
membox                          # snapshot (like git commit)
membox -m "before big refactor" # snapshot with a label
membox log                      # history (like git log)
membox diff 2026-05-16          # what changed (like git diff)
membox restore "Design Doc"     # find & recover a file
membox status                   # drive health
membox prune --days 90          # clean old archives
```

## How it works

- **First run:** full copy of everything to the external drive
- **Subsequent runs:** incremental — only changed files transferred
- **Deleted/overwritten files:** moved to a dated archive, never lost
- **Manifests:** every snapshot records each file's path, size, and mtime — enables O(n) diff without loading the full list into memory

## Install

### Homebrew (recommended)

```bash
brew install up1512001/tap/membox
```

### Build from source

```bash
git clone https://github.com/up1512001/memorybox
cd memorybox
go build -o /usr/local/bin/membox ./cmd/memorybox
```

Requires Go 1.22+ and `rsync` on `$PATH` (ships with macOS).

## Usage

### `membox` — run a backup

```bash
membox                           # backup all sections
membox -m "before deploy"        # add a label
membox --sections photos,docs    # specific sections only
membox --dry-run                 # preview without changes
membox --parallel 4              # increase concurrent sections
membox --quiet                   # silent (for cron/launchd)
```

**Sections:** `photos` `movies` `docs` `desktop` `downloads` `icloud` `dev` `localsites` `config`

### `membox log` — view history

```bash
membox log                       # last 10 snapshots
membox log -n 30                 # last 30
membox log --all                 # all snapshots
membox log --oneline             # compact format
```

### `membox diff` — compare snapshots

```bash
membox diff                      # latest vs previous
membox diff 2026-05-16           # specific date (partial match ok)
membox diff 2026-05-16T14-30-00Z # exact snapshot key
membox diff --stat               # counts only
membox diff --name-only          # changed file paths
membox diff --name-status        # A/D/M prefix + path
```

### `membox restore` — find & recover files

```bash
membox restore "invoice"         # search across all archives
membox restore "*.pdf"           # glob pattern
membox restore "Design Doc" --to ~/Desktop/recovered
membox restore "logo" --snapshot 2026-04-01
membox restore "file" --dry-run  # show matches only
```

### `membox status` — drive health

```bash
membox status
```

```
Backup Status
─────────────────────────────────────────────────────────
Drive:     /Volumes/X10 Pro  [████████████░░░░░░░░░░░░░░░░░░] 41% used
           478.3G free of 953.9G total

Last backup:  2026-05-16T14-30-00Z — today at 14:30 — "before deploy"
Snapshots:    12 total
```

### `membox prune` — clean old archives

```bash
membox prune                     # use config defaults (90 days, keep 8)
membox prune --days 60           # delete archives older than 60 days
membox prune --keep 4            # always keep last 4 snapshots
membox prune --dry-run           # show what would be deleted
membox prune --force             # skip confirmation
```

## Configuration

Config file (optional): `~/.config/memorybox/config.yaml`

```yaml
drive:
  mountPath: /Volumes/X10 Pro      # external SSD mount point

parallel: 2                        # concurrent rsync sections

prune:
  defaultDays: 90
  defaultKeep: 8

sections:
  dev:
    excludes:
      - node_modules
      - vendor
      - .git
      - dist
```

**Override priority:** CLI flags → `MEMBOX_*` env vars → config file → defaults.

Environment variable examples:
```bash
MEMBOX_DRIVE_MOUNTPATH=/Volumes/MyDrive membox status
MEMBOX_PARALLEL=4 membox
```

## Drive layout

```
/Volumes/X10 Pro/
├── backup-current/          # live mirror (rsync destination)
├── backup-archive/
│   ├── 2026-05-16T14-30-00Z/  # files deleted/overwritten this snapshot
│   └── 2026-05-09T10-15-00Z/
├── backup-manifests/
│   ├── 2026-05-16T14-30-00Z.manifest
│   └── 2026-05-09T10-15-00Z.manifest
└── backup-logs/
    ├── 2026-05-16T14-30-00Z.log
    └── history.csv
```

## Sections backed up

| Section | Source | Notes |
|---------|--------|-------|
| `photos` | `~/Pictures` | |
| `movies` | `~/Movies` | |
| `docs` | `~/Documents` | |
| `desktop` | `~/Desktop` | |
| `downloads` | `~/Downloads` | |
| `icloud` | `~/Library/Mobile Documents/com~apple~CloudDocs` | |
| `dev` | `~/Developer` | excludes node_modules, .git, dist, etc. |
| `localsites` | `~/Local Sites` | excludes same as dev |
| `config` | dotfiles + inventory | .zshrc, .gitconfig, .ssh, VS Code, brew list, etc. |

## Requirements

- macOS (Intel or Apple Silicon)
- Go 1.22+ (build only)
- `rsync` — ships with macOS, no install needed
- External SSD formatted as APFS or exFAT

## Development

```bash
git clone https://github.com/up1512001/memorybox
cd memorybox

go build -o ./bin/membox ./cmd/memorybox   # build
go test ./...                               # tests
go test -bench=. -benchmem ./internal/manifest/...  # benchmarks
```

See [`AGENTS.md`](AGENTS.md) for codebase guide (architecture, conventions, what not to do).

## License

MIT — see [LICENSE](LICENSE).
