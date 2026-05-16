## Summary

<!-- What does this PR do? 1-3 bullet points. -->

- 

## Motivation

<!-- Why is this change needed? Link related issues with `Closes #N`. -->

## Test plan

<!-- How did you verify this works? Check all that apply. -->

- [ ] `go test ./...` passes
- [ ] `go test -race ./...` passes
- [ ] `GOOS=linux go build ./...` passes (Linux compat)
- [ ] Manually tested with a real backup run
- [ ] `membox diff` output looks correct before/after
- [ ] `--dry-run` behaves correctly (no files written)

## Breaking changes

<!-- Does this change the manifest format, config schema, or CLI flags? -->

None / describe here.

## Checklist

- [ ] No `fmt.Printf` in command handlers — all output via `a.Printer.*`
- [ ] Manifest writes remain sorted by path
- [ ] No `ReadAll` on manifest files — streaming only
- [ ] New OS-specific code uses build tags (`_darwin.go` / `_linux.go`)
- [ ] No new CGO dependencies
