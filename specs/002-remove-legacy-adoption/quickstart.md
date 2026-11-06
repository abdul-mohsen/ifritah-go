# Quickstart Validation: Remove Legacy Database Adoption

Run these commands from
`C:\ssda\chatGPT\ifritah\backend.git\worktree\backend-b5-atomic-sale`.

## Static checks

```powershell
rtk git --no-pager status --short
rtk git diff --check
rtk gofmt -l .
rtk git ls-files pkg/db/goose
```

Expected results:

- Only issue #123 changes and feature artifacts are present.
- No whitespace errors or unformatted Go files are reported.
- The migration directory contains exactly `00001` through `00010` SQL files
  and no adoption/digest support files.

## Focused migration tests

```powershell
rtk go test ./pkg/db/goose/...
```

With `GOOSE_PARITY_DSN` configured, MySQL-backed baseline, migration,
tenant-safety, and round-trip tests execute. Without it, those tests skip and
the static migration tests still run.

## Repository checks

```powershell
rtk go test ./...
rtk go build ./...
rtk go vet ./...
rtk gofmt -l .
```

All commands must exit successfully. No command in this guide invokes a
remote or GitHub write.
