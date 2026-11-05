# STATUS

## Completed

- T1: Requirements and repository research are complete.
- The authoritative request-identity constraint is `uq_bill_pos_request`.
- The existing `sqlmock` sale-handler harness is suitable for both regression
  cases.
- T2: Added the sale-specific named-key duplicate classifier without changing
  shared duplicate behavior.
- T3: Updated true-race injections to use the real key, added the unrelated-key
  handler regression, and added direct classifier coverage.
- T4: Focused identity tests pass (7 tests); the full backend suite passes
  (238 tests across 12 packages); the backend build passes; changed-file
  formatting and `git diff --check` pass.
- T5: Self-review confirms only the sale update classifier and its regressions
  changed. The shared duplicate helper, schema, transaction flow, and other
  reviewer findings are untouched.
- The root README was reviewed; this internal error-classification correction
  needs no user-facing documentation update.

## In Progress

- None.

## Blocked

- Repository-wide `rtk go vet ./...` reports the pre-existing generated-code
  issue `pkg/db/gen/models.go:1007: struct field _0 has json tag but is not
  exported`. The changed handler package reaches the same dependency warning;
  fixing generated code is outside this finding and was not attempted.
- Repository-wide `rtk gofmt -l .` lists pre-existing unformatted files; both
  changed Go files produce no output from targeted `gofmt -l`.

## Backlog

- None.

## Risks

- MySQL duplicate messages must retain the named constraint identifier; the
  regression will use the same driver message shape as the schema parity test.
- The shared `IsDuplicate` helper is used by unrelated handlers and must not be
  changed.

## Next Actions

- Await designated QA review; no commit or push is planned.
