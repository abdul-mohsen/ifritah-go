# Tenant database migrations (goose)

`00001_baseline.sql` is the squashed tenant schema. It replaces the legacy chain
`pkg/db/schema/schema.sql` + `pkg/db/migrations/*.sql` + `pkg/db/runtime_migrations/*.sql`
for new tenant databases. Existing tenant databases are adopted at version 1 by the
deployment change that follows this PR, so goose never re-runs the baseline.

The legacy folders still drive `sqlc generate` and the existing replay check
(`.github/workflows/mysql-schema.yml`) until the deployment tooling moves to goose.
`pkg/db/gen` is git-ignored: `go.yml` regenerates it before every build.

## Commands

Run from the repository root. The database must already exist.

```sh
HOST=127.0.0.1:3306 DBUSER=root PASSWORD=root DBNAME=my_tenant go run ./cmd/migrate up
go run ./cmd/migrate status     # list every migration and whether it is applied
go run ./cmd/migrate down       # roll back one migration; refuses to go below the baseline
go run ./cmd/migrate roundtrip  # roll back everything above the baseline, then re-apply
go run ./cmd/migrate version
```

## Rules for new migrations

- File name `NNNNN_snake_case.sql` with the next free five-digit version. sqlc reads
  migration folders in name order, so the zero padding matters.
- Each file has `-- +goose Up` and `-- +goose Down`. The Down section must reverse the Up section.
- Views and other stored objects must not name a fixed `DEFINER`. Use `DEFINER=CURRENT_USER` or omit it, so a tenant account can create them.
- CI runs `roundtrip` against MySQL 8.0.28, 8.0.44 and 8.4. A migration whose Down does not
  work fails the pull request.

## What CI checks

- `go test ./pkg/db/goose/...`: file names and versions, a Down section for every migration
  after the baseline, and (when `GOOSE_PARITY_DSN` is set) a comparison of the baseline with the
  legacy SQL, table by table.
- `cmd/migrate up`, `status`, `roundtrip`, `up`, `version` on a fresh database.
- The baseline replayed by a tenant account that has only `ALL` on its own database. `ALL` does not include `SUPER` or `SET_USER_ID`.
