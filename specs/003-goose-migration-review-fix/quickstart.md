# Quickstart validation

## Without MySQL

From the repository root:

```sh
go test -run 'TestMigrationFiles|TestBillCounterDownIsNonDestructive' ./pkg/db/goose
gofmt -l cmd/migrate/main.go pkg/db/goose/*.go
```

Expected: migration files are contiguous from 00001 through 00010, the 00008 Down section
contains no destructive `branch_sequence` operation, and `gofmt -l` prints nothing.

## With MySQL

Set `GOOSE_PARITY_DSN` to a root-capable DSN that points at the server (without selecting a
tenant database), then run:

```sh
go test -count=1 -run 'Test(MigrationFiles|BillNumberCounter|Adopt)' ./pkg/db/goose
```

Expected:

- a fresh database migrates through the numbered goose files;
- existing bills seed counters, lower counters are raised, higher counters are retained, and
  null sequences do not create false counters;
- Down preserves pre-existing bill and non-bill sequence rows; and
- an exact pre-goose baseline can be adopted while empty, drifted, and already-managed
  databases are refused.
- the external daemon's reservation SQL contract serializes bookings and rejects stale
  release/commit re-deliveries.

For an operator-managed database, use:

```sh
HOST=127.0.0.1:3306 DBUSER=root PASSWORD=... DBNAME=my_tenant go run ./cmd/migrate adopt
HOST=127.0.0.1:3306 DBUSER=root PASSWORD=... DBNAME=my_tenant go run ./cmd/migrate up
```

Only run `adopt` after independently confirming that the database is the pre-goose baseline;
the command performs the structural digest check and fails closed on drift.
