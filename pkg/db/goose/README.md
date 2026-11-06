# Tenant database migrations (goose)

`00001_baseline.sql` is the tenant schema. A fresh tenant database is built from it by
`migrate up`. The migration workflow is fresh-system-only; pre-Goose tenant databases are not
supported by this command.

`sqlc.yaml` reads this folder for the main tenant schema, so every goose migration reaches the generated code. `pkg/db/schema/car_part.sql` stays a separate sqlc input: it is another database, and sqlc reads it first (see the comment in `sqlc.yaml`).
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

The compose stack runs `/app/migrate up` once, in its `migrate` service, before the API starts.
CI runs `go run ./cmd/migrate up`. Deployment moves to the same command in DP1.

## Rules for new migrations

- New schema changes go only in this folder. sqlc reads it, so no other file needs to change.
- File name `NNNNN_snake_case.sql` with the next free five-digit version. sqlc reads
  migration folders in name order, so the zero padding matters.
- Each file has `-- +goose Up` and `-- +goose Down`. The Down section must reverse the Up section.
- A data backfill that cannot identify its own rows may use a documented non-destructive Down;
  `00008_bill_number_counter.sql` deliberately preserves counters to prevent number reuse.
- `00005_egs_chain_reservation.sql` retains the reservation columns for `queue.SQLChain` in the
  external ZATCA daemon. The backend no longer owns booking, release, or commit SQL for that chain.
- Views and other stored objects must not name a fixed `DEFINER`. Use `DEFINER=CURRENT_USER` or omit it, so a tenant account can create them.
- CI runs `roundtrip` against MySQL 8.0.28, 8.0.44 and 8.4. A migration whose Down does not
  work fails the pull request.

## What CI checks

- `go test ./pkg/db/goose/...`: file names and versions, a Down section for every migration
  after the baseline.
- the bill-counter backfill and its non-destructive Down on a MySQL parity database;
- the `queue.SQLChain`-owned EGS reservation SQL contract (booking serialization, lease takeover,
  owner guards, commit, release, and stale re-delivery);
- `cmd/migrate up`, `status`, `roundtrip`, `up`, `version` on a fresh database.
- `.github/scripts/test-sqlc-goose.sh` (sqlc 1.27.0, the CI version): a probe migration with a Down section reaches the generated model, and the Down is ignored.
- The baseline replayed by a tenant account that has only `ALL` on its own database. `ALL` does not include `SUPER` or `SET_USER_ID`.
