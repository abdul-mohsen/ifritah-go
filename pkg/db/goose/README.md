# Tenant database migrations (goose)

`00001_baseline.sql` is the tenant schema. Every tenant database is built from it by `migrate up`. There is one migration path: no legacy SQL and no adoption.

`sqlc.yaml` reads this folder for the main tenant schema, so every goose migration reaches the generated code. `pkg/db/schema/car_part.sql` stays a separate sqlc input: it is another database, and sqlc reads it first (see the comment in `sqlc.yaml`).
`pkg/db/gen` is git-ignored: `go.yml` regenerates it before every build.

The EGS reservation columns in `00005_egs_chain_reservation.sql` are daemon-owned
storage for `queue.SQLChain`; the backend migration does not implement runtime
booking, lease takeover, release, or commit. The MySQL contract test runs when
`GOOSE_PARITY_DSN` is set.

## Commands

Run from the repository root. The database must already exist.

```sh
HOST=127.0.0.1:3306 DBUSER=root PASSWORD=root DBNAME=my_tenant go run ./cmd/migrate up
go run ./cmd/migrate status     # list every migration and whether it is applied
go run ./cmd/migrate down       # roll back one migration; refuses to go below the baseline
go run ./cmd/migrate roundtrip  # roll back everything above the baseline, then re-apply
go run ./cmd/migrate version
```

The compose stack runs `/app/migrate up` once, in its `migrate` service, before the API starts. CI runs `go run ./cmd/migrate up`. Deployment moves to the same command in DP1.

## Rules for new migrations

- New schema changes go only in this folder. sqlc reads it, so no other file needs to change.
- File name `NNNNN_snake_case.sql` with the next free five-digit version. sqlc reads
  migration folders in name order, so the zero padding matters.
- Each file has `-- +goose Up` and `-- +goose Down`. The Down section must reverse the Up section.
- Views and other stored objects must not name a fixed `DEFINER`. Use `DEFINER=CURRENT_USER` or omit it, so a tenant account can create them.
- CI runs `roundtrip` against MySQL 8.0.28, 8.0.44 and 8.4. A migration whose Down does not
  work fails the pull request.

## What CI checks

- `go test ./pkg/db/goose/...`: file names and versions, a Down section for every migration
  after the baseline.
  
- `cmd/migrate up`, `status`, `roundtrip`, `up`, `version` on a fresh database.
- `.github/scripts/test-sqlc-goose.sh` (sqlc 1.27.0, the CI version): a probe migration with a Down section reaches the generated model, and the Down is ignored.
- The baseline replayed by a tenant account that has only `ALL` on its own database. `ALL` does not include `SUPER` or `SET_USER_ID`.
