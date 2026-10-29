# T016 Status

## Completed

- T016A: Baseline commit, existing global-max query, branch allocator,
  branch-ZATCA fields, T014 envelope/outbox, and credit/debit limitations
  researched.
- T016A: Requirements, research, design, plan, tasks, data model, daemon
  contract, spec, and quickstart artifacts created before source edits.
- T016B: Red evidence captured with
  `rtk proxy go test ./pkg/handlers -run '^TestAddBillEnqueuesBeforePublication$' -count=1`.
  The current handler called `GetMaxSequenceNumber` and failed the expected
  branch allocator query.
- T016D: `AddBill` and `SubmitDraftBill` now use the existing atomic
  `AllocateBranchSequence` query with `branch_id` and `bill` scope; the global
  max query was removed.
- T016E: Added the additive `zatca_egs_chain` migration and sqlc contract for
  ensure/lock/claim/commit/release with branch/EGS/owner predicates.
- T016F: Added branch isolation/concurrency and owner-mismatch sqlmock
  regressions, updated README, and preserved T014 outbox/envelope behavior.
- T016G: `rtk sqlc generate`, `rtk gofmt`, focused package tests, and
  `rtk go test ./... -count=1` passed (126 tests in 6 packages).
- T016G: `rtk go build -o t016-build.exe .` passed after removing the temporary
  project-local binary.
- T016H: Final scope review confirms no daemon XML-chain, debit-domain, or
  outbox/envelope identity changes.

## In Progress

- None.

## Blocked

- No blocker currently.

## Backlog

- Live MySQL verification of InnoDB row locks, transaction isolation,
  deadlocks, and lease expiry remains required before production rollout.
- T017/T018 daemon XML/hash-chain work remains intentionally separate.

## Risks

- sqlmock cannot prove real row-lock serialization or deadlock behavior.
- Existing credit-note storage has no independent sequence column and debit
  domain is absent; T016 exposes shared chain ownership without adding those
  domains.
- Generated sqlc output is ignored and must be regenerated in every validation
  environment.
- Full `rtk go build ./...` could not remove a Windows Go temporary
  `a.out.exe` (`unlinkat ... process cannot access the file`) after repeated
  attempts; explicit-output root compilation passed.
- Full `rtk go vet ./...` reports the pre-existing generated sqlc field
  `pkg/db/gen/models.go:994` (`_0` has a JSON tag but is unexported). Generated
  output is not hand-edited.

## Next Actions

1. Before deployment, run the daemon contract against live MySQL to verify
   InnoDB lock, isolation, deadlock, and lease-expiry behavior.
