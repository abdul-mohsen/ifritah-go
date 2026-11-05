# Status

## Completed

- T1: Scoped requirements and repository-history research completed.
- Confirmed current goose files are numbered 00001 through 00010.
- Confirmed 00008 was introduced in repository history and must not be renumbered without
  deployment evidence.
- Inspected the requested worktree: no current daemon SQL or backend consumer remains; deployment
  evidence identifies `queue.SQLChain` as the runtime owner of the reservation columns.
- T2: Restored explicit digest-checked adoption and command coverage.
- T3: Made 00008 Down non-destructive.
- T4: Added bill-counter, migration-owner, and EGS SQL-contract regression fixtures.
- T5: Updated migration README and quickstart.
- T8: Established `queue.SQLChain` as the sole EGS reservation owner.
- T7: Completed self-review and local commit `54d5f70`.

## In Progress

- None.

## Blocked

- Live MySQL parity is unavailable because `GOOSE_PARITY_DSN` is unset.

Validation completed without a DSN: focused migration tests passed (3 static tests; database
tests skipped), `go test -count=1 ./...` passed (240 tests, 12 packages), `go build ./...`
passed, `gofmt` reported no changed files, and `go vet ./cmd/migrate` passed. Full `go vet
./...` remains blocked by the pre-existing generated-model warning recorded below.

## Backlog

- Run the MySQL parity suite in CI or an environment with `GOOSE_PARITY_DSN`.

## Risks

- Adoption tests and counter rollback tests require MySQL and will skip without
  `GOOSE_PARITY_DSN`.
- A drifted external tenant still needs operator repair; this change intentionally does not
  auto-repair schema.
- The daemon source is outside the requested worktree, but deployment evidence identifies
  `queue.SQLChain`; removing the columns would be unsafe without a coordinated daemon change.
- `go vet ./...` reports a pre-existing generated-model warning at `pkg/db/gen/models.go:1007`
  (`_0` has a JSON tag); `go vet ./cmd/migrate` passes.

## Next Actions

- Run the designated QA review when a QA process is available; rerun MySQL parity in CI or an
  environment with `GOOSE_PARITY_DSN`.
