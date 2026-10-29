# T024 Quickstart Validation

## Prerequisites

- Go toolchain and repository dependencies installed.
- `sqlc` available on `PATH`.
- No live MySQL, NATS, or ZATCA provider is required for focused tests.

## Focused regression

From the backend worktree:

```powershell
rtk go test ./pkg/handlers ./pkg/db -run 'Debit|Zatca' -count=1
```

Expected: debit source/API/publisher/monitor regressions pass and existing
outbox/envelope/monitor regressions remain green.

## Generation and repository validation

```powershell
rtk sqlc generate
rtk gofmt -w pkg/handlers pkg/model
rtk go build .
rtk go vet ./...
rtk go test ./... -count=1
rtk git diff --check
```

The generated `pkg/db/gen` files are pipeline output and should not be edited
or committed.

## Manual MySQL follow-up

Apply migrations `0011_zatca_debit_note.sql` and
`0012_zatca_submission_sources.sql` to a representative tenant copy. Verify
historical submission backfill, bill/credit/debit uniqueness, foreign keys,
online DDL behavior, and monitor rows after a daemon/provider outcome. Record
the live evidence separately; sqlmock cannot prove these properties.
