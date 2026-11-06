# Design: Remove Legacy Database Adoption

## Architecture and Boundaries

The migration package remains the sole owner of the embedded Goose migration
set. The command package continues to create a Goose provider and expose the
supported operational commands:

```text
cmd/migrate
  └── goose.Provider(FS)
        └── pkg/db/goose/*.sql
```

The removed adoption boundary is intentionally not replaced. Fresh tenant
databases enter through `migrate up`; pre-Goose database conversion is outside
the supported system.

## Changes

1. Delete `pkg/db/goose/adopt.go`.
2. Delete `pkg/db/goose/schema_digest.go` and
   `pkg/db/goose/baseline.digests`.
3. Delete `pkg/db/goose/adopt_test.go`, including parity and digest-generation
   tests that exist only for adoption.
4. Remove the `adopt` command branch/helper/help text from
   `cmd/migrate/main.go`, preserving all other commands and connection
   behavior.
5. Update `pkg/db/goose/README.md`, `pkg/db/goose/migrations.go`, and the
   baseline header in `00001_baseline.sql` so the documented workflow is
   fresh-system-only without changing SQL statements or migration numbering.
6. Leave migration SQL, `BaselineVersion`, tenant replay CI, and unrelated
   build/image digest code unchanged.

## Migration Behavior

- `00001_baseline.sql` remains the baseline migration and the first applied
  version on a fresh database.
- Migrations `00002` through `00010` remain unchanged and contiguous.
- `down` and `roundtrip` continue to stop at `BaselineVersion` and never
  remove the baseline schema.
- `up`, `status`, and `version` continue to use the same Goose provider and
  embedded filesystem.

## Failure Handling and Safety

- With no supported adoption path, operators of pre-Goose databases must use
  the fresh-system provisioning policy; no automatic schema guessing or
  version-table writes are performed.
- Existing tenant-account replay validation remains in CI and continues to
  exercise the baseline with only database-local privileges.
- No new SQL or privilege is introduced, so migration security posture is
  unchanged.

## Test Strategy

- Run the focused migration package tests first:
  `go test ./pkg/db/goose/...`.
- Run the complete repository tests, build, vet, and formatting checks required
  by the task.
- Verify migration filenames and stale adoption references with repository
  searches.
- If MySQL-dependent tests skip because `GOOSE_PARITY_DSN` is absent, record
  that fact; do not weaken or delete the tests that still provide coverage.

## Documentation and Rollout

The migration README will state that `migrate up` builds a fresh tenant and
that pre-Goose database adoption is unsupported. No deployment rollout or
database data migration is provided because the policy intentionally removes
the compatibility path.

## Requirement Traceability

| Requirement | Design response |
|---|---|
| FR-001/FR-002 | Remove the CLI entry point and adoption/digest files. |
| FR-003 | Keep all ten numbered SQL files and `BaselineVersion = 1`. |
| FR-004/FR-005 | Preserve Goose provider, `up`, and baseline rollback boundary. |
| FR-006 | Preserve tenant replay workflow and baseline SQL. |
| FR-007/FR-008 | Rewrite adoption docs only; leave unrelated image/build digests. |
| NFR-001 | Execute focused and full Go validation plus formatting. |
| NFR-002/NFR-003 | Work only in this worktree; do not touch migration SQL behavior or remote systems. |
