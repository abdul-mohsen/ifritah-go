# Quickstart: Validate Fresh Tenant Defaults

Run from
`C:\ssda\chatGPT\ifritah\backend.git\worktree\backend-b13-check-2`.

## Without MySQL

The migration file and its Goose metadata checks run without a database:

```text
rtk go test -count=1 ./pkg/db/goose/...
```

The fresh-database tests skip when `GOOSE_PARITY_DSN` is unset.

## With MySQL

Set a DSN that can create and drop test databases, then run:

```text
GOOSE_PARITY_DSN="root:root@tcp(127.0.0.1:3306)/?multiStatements=true&parseTime=true" rtk go test -count=1 ./pkg/db/goose/...
```

Expected behavior:

- A fresh migrated database contains all seven required settings.
- A custom WhatsApp API version and markup percentage remain unchanged.
- Rolling migration 11 down and up leaves one row per key with the same values.

## Repository Checks

```text
rtk go test -count=1 ./...
rtk go build ./...
rtk go vet ./cmd/migrate
rtk gofmt -l pkg/db/goose/settings_test.go
```

The repository currently has a pre-existing generated-model vet warning when
running `go vet ./...`; it is unrelated to this migration.
