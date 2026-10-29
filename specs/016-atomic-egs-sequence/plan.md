# T016 Implementation Plan

## Technical Context

- Language/runtime: Go 1.25 with the repository's Go 1.26.5 toolchain.
- Persistence: MySQL 8/InnoDB, `database/sql`, sqlc-generated Go package.
- Existing boundary: tenant-specific database connection; `branch_id` and EGS
  identity are query parameters.
- Existing allocator: `branch_sequence` and `AllocateBranchSequence`.
- Existing event boundary: T014 versioned ZATCA outbox envelope.
- Testing: Go tests with `go-sqlmock`; no live MySQL is assumed.

## Constitution Check

No project constitution exists in the baseline `.specify` directory. T016
therefore applies the repository's observable constraints directly: preserve
generated SQL workflow, use parameterized SQL, keep tenant boundaries explicit,
avoid daemon/domain expansion, and validate before commit. No application
constitution file is added because it would be unrelated governance scope.

## Phase 0: Research

Completed in `research.md`. The global max query, existing branch allocator,
branch ZATCA fields, T014 envelope, credit/debit limitations, and sqlc
generation workflow were inspected from the baseline commit.

## Phase 1: Design

Completed in `design.md`, `data-model.md`, `contracts/zatca-chain.md`, and
`quickstart.md`.

## Phase 2: Implementation

1. Add the additive chain-state migration and source schema definition.
2. Add sqlc queries for branch/EGS reservation lifecycle.
3. Replace posted-bill global max allocation with branch allocator and
   propagate allocation errors.
4. Add focused sqlmock/concurrency and ownership tests.
5. Add additive event metadata only if the existing outbox construction can
   source it without changing T014 identity or publication semantics.
6. Update README and status evidence.

## Phase 3: Validation

Run in order:

```powershell
rtk go test ./pkg/handlers -run 'Sequence|Chain|Outbox'
rtk sqlc generate
rtk gofmt -w <changed Go files>
rtk go build ./...
rtk go vet ./...
rtk go test ./...
rtk git diff --check
```

Record the initial red test, every green command, and the live-MySQL
limitation in `STATUS.md`.
