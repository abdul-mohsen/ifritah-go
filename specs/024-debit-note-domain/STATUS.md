# T024 Status

## Completed

- T024A: Requirements, research, design, spec, plan, tasks, and status
  artifacts created before implementation.
- T024B: Added focused debit API/publisher/monitor regressions and captured the
  red baseline at `b602c05`.
- T024C: Added replay-safe debit source and generic submission migrations,
  final schema source, sqlc debit queries, and monitor source fields.
- T024D: Added JWT-protected debit creation, pending submission registration,
  transactional outbox enqueue, and commit-before-wake behavior.
- T024E: Added durable debit publication context and additive envelope
  sequence field while preserving existing event identity.
- T024F: Added additive monitor document/source/sequence/EGS/stable-key fields
  and tenant-safe scan regressions.

## In Progress

- T024H: final diff/self-review and isolated commit.

## Blocked

- None currently.

## Backlog

- Live MySQL migration/unique-key/locking validation.
- Later daemon debit XML, provider submission, and outcome processing.

## Risks

- sqlmock cannot prove online MySQL DDL, InnoDB isolation, deadlocks, or
  daemon/provider outcome persistence.
- Generic submission migration must preserve historical bill rows while
  allowing distinct correction-document source identities.
- Generated sqlc output is ignored and must be regenerated in every validation
  environment.
- `go vet ./...` reports the pre-existing generated sqlc warning
  `pkg/db/gen/models.go:1000` (`_0` has a JSON tag but is unexported); generated
  output was not hand-edited.
- No live MySQL instance was available, so online migration, unique-key
  backfill, InnoDB locking/isolation, and daemon/provider outcome writes were
  not exercised.

## Next Actions

1. Create the isolated T024 commit after final scope review; do not push or
   create a PR.

## Validation Evidence

- Red baseline: focused debit tests failed at `b602c05` because the debit API,
  contextual publisher builder, envelope sequence field, and monitor source
  fields were absent.
- Green focused: isolated handler/database binaries passed focused
  `Debit|Zatca` tests.
- Green package: complete `pkg/handlers` and `pkg/db` package binaries passed.
- Green full: `rtk proxy go test -C C:\ssda\chatGPT\worktrees\zatca-fix-t024-backend ./... -count=1` passed.
- SQL/format/build: `rtk sqlc generate`, `gofmt`, and `go build -o t024-build.exe .` passed; temporary binaries were removed.
- Vet: `go vet ./...` reached the pre-existing generated `models.go:1000` warning above.
- Diff: `git diff --check` passed.
