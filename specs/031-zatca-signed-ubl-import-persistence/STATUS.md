# T031 Status

## Completed

- T031-REQ: Requirements, research, design, spec, plan, tasks, and status
  artifacts created before source implementation.
- Baseline confirmed: branch `fix/zatca/m4-import-persistence` at
  `98ae215a062c0aa9b2defe77b3ab1522f718bd30`.
- T030 contract read from separate repository commit `b0099f7`.
- T031-RED: Added focused create/review/post/reject sqlmock regressions;
  baseline compile failed because import models/handlers did not exist.
- T031-SCHEMA: Added migration `0013_zatca_signed_ubl_import.sql`, checked-in
  schema definitions, and sqlc query source; `sqlc generate` passed.
- T031-API: Added immutable evidence/status/review persistence and JWT-routed
  create/list/detail/review/post/reject APIs without outbound writes.
- T031-GREEN: Focused import and existing ZATCA regression packages pass.
- T031-DOC: README documents the authenticated import routes and explicit
  outbound/daemon boundary.
- T031-VALIDATE: `sqlc generate`, `gofmt`, build, focused tests, full
  `go test ./... -count=1`, and `git diff --check` pass. `go vet ./...`
  reaches only the pre-existing generated sqlc warning at
  `pkg/db/gen/models.go:1000` (`_0` has a JSON tag but is unexported).
- T031-CONVERGE: Final diff review confirms no frontend or daemon files and no
  writes to outbound submission/outbox tables.

## In Progress

- T031-COMMIT: Create the isolated local T031 commit; do not push or create a
  PR.

## Blocked

- None currently.

## Backlog

- Live MySQL migration/locking/JSON/MEDIUMBLOB rehearsal.
- Live object-storage/reference retention verification, if deployment supplies
  an external storage adapter later.

## Risks

- sqlmock cannot prove MySQL DDL, foreign-key/index behavior, row-locking,
  transaction isolation, or MEDIUMBLOB/JSON limits.
- The backend intentionally accepts T030-shaped evidence read-only and cannot
  prove that an upstream caller actually ran the T030 parser.
- Storing source bytes in MySQL is bounded by the T030 five-megabyte input
  limit; production may later need a blob-store adapter while retaining the
  hash/reference contract.

## Next Actions

1. Commit the bounded T031 implementation and return the SHA.

## Validation Evidence

- Red baseline: focused tests failed before implementation because
  `CreateZatcaImport`, lifecycle handlers, and T030-aligned models were absent.
- Green focused: `rtk proxy go test -C C:\ssda\chatGPT\worktrees\zatca-fix-t031-backend ./pkg/handlers -run 'TestZatcaImport' -count=1 -vet=off`.
- Green full: `rtk proxy go test -C C:\ssda\chatGPT\worktrees\zatca-fix-t031-backend ./... -count=1`.
- SQL/format/build: `rtk sqlc generate`, `gofmt`, `rtk go build ...`, and
  `git diff --check` pass.
- Vet limitation: `go vet ./...` exits non-zero on the pre-existing ignored
  generated warning in `pkg/db/gen/models.go:1000`; generated output was not
  hand-edited.
- Live limitations: no live MySQL or object-storage service was available;
  migration DDL, InnoDB locking/isolation, JSON/MEDIUMBLOB limits, and
  external reference retention require deployment rehearsal.
