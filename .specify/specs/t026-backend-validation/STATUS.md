# T026 Status

## Completed

- T026-A: requirements and repository/contract research recorded.
- T026-B: design, plan, task dependencies, and traceability recorded.
- T026-C: added `TestOnboardBranchZatcaRejectsNonASCIIDigitsBeforePublishing`
  and confirmed the baseline red result.
- T026-D: added strict OTP/config validation, an existing-query gate, a
  no-publication test seam, and focused green tests.
- T026-E: updated the backend README, formatted the changed Go files, and ran
  the targeted and full test suites.
- T026-F: completed final self-review, confirmed a T026-only clean worktree,
  and created the final T026 commit.

## In Progress

- None.

## Blocked

- Repository-wide `go vet` is blocked by the pre-existing ignored sqlc
  generated warning in `pkg/db/gen/models.go:994:2` (`struct field _0 has json
  tag but is not exported`). Fixing generated vehicle-fitment code is outside
  T026; targeted and full Go tests pass.

## Backlog

- None.

## Risks

- Strict onboarding validation may reject legacy incomplete rows; this is
  intentional and will be surfaced with field-level details.
- Generated sqlc sources are ignored and must be regenerated locally for tests;
  no generated file should be committed.

## Next Actions

1. No further T026 implementation action remains; do not push or create a PR
   from this lane.

## Validation Evidence

- Red command:
  `rtk go test ./pkg/handlers -run 'TestOnboardBranchZatcaRejectsNonASCIIDigitsBeforePublishing' -count=1`
- Red result on `9bdb0d6`: failed because the six-character malformed OTP
  reached the nil publisher path (`runtime error: invalid memory address`).
- Green focused command:
  `rtk go test ./pkg/handlers -run 'TestOnboardBranchZatca' -count=1`
- Green full command:
  `rtk go test ./... -count=1`
- Formatting command:
  `rtk gofmt -w pkg/handlers/branch.go pkg/handlers/handler.go
  pkg/handlers/zatca_validation.go pkg/handlers/zatca_validation_test.go`
- `rtk go vet ./...` and `rtk go vet ./pkg/handlers` remain blocked by the
  pre-existing ignored sqlc-generated warning
  `pkg/db/gen/models.go:994:2: struct field _0 has json tag but is not exported`.
