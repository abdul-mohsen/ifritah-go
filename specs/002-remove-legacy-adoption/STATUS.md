# STATUS

## Completed

- Requirements, research, design, spec, checklist, plan, data model,
  quickstart, and dependency-ordered tasks are written for issue #123.
- Repository inspection identified the complete adoption surface and confirmed
  migration numbering, baseline rollback safety, and tenant replay coverage.
- Task graph validation confirms T004-T010 are ready only after the
  foundational migration-invariant check.
- T003-T010 removed the adoption implementation, digest support, CLI command,
  stale docs, and stale references without changing migration SQL behavior.
- T011 passed the focused migration package tests: three static tests passed
  and seven MySQL-backed tests skipped because `GOOSE_PARITY_DSN` is unset.
- T012 passed `go test ./...` (240 tests, 12 packages) and `go build ./...`.
  `go vet -tests=false ./...` passed; full `go vet ./...` reports the
  pre-existing ignored generated field `pkg/db/gen/models.go:1007` (`_0` has a
  JSON tag but is unexported). Changed Go files are formatted; repository-wide
  `gofmt -l .` reports pre-existing unformatted files.
- T013-T014 found no forbidden adoption symbols in `cmd`, `pkg`, or `.github`;
  migration files remain exactly `00001` through `00010`, and the diff review
  preserves tenant replay and `DEFINER=CURRENT_USER`.
- T015 completed: todo `removing-legacy-adoption` is `done`, a local commit
  was created, and no push or GitHub write was performed.
- Final convergence/self-review found no remaining scoped gaps between the
  requirements, plan, tasks, and implementation.

## In Progress

- None.

## Blocked

- None currently.

## Backlog

- T011: Focused migration validation.
- T012-T013: Full validation and stale-reference search.
- T014-T015: Self-review, todo update, QA evidence, and local commit.

## Risks

- MySQL-backed migration tests may skip if `GOOSE_PARITY_DSN` is not set.
- Broad digest searches include unrelated image/build metadata; those must
  remain unchanged.
- The designated QA agent has not yet registered an idle lease.
- Full repository vet and gofmt are not clean due to pre-existing generated
  code and unrelated files; focused changed-file formatting and non-test vet
  pass.
- The feature-chat MCP intermittently returned `FEATURE_NOT_FOUND` after the
  chat was opened, so acceptance evidence could not be recorded there and a
  QA approval could not be obtained in this session.

## Next Actions

- No implementation actions remain. Parent/QA can review the local commit and
  the documented pre-existing validation caveats.
