# T010 Status

## Completed

- T010-REQ: requirements recorded for durable onboarding state, attempt,
  progress, retry/error, completion, queued transition, and strict validation.
- T010-RESEARCH: repository, dependent daemon/client, migration, transaction,
  and generated-code evidence recorded.
- T010-DESIGN: persistence, query, handler, failure, security, and test design
  recorded.
- T010-RED: added focused sqlmock regressions and captured the pre-implementation
  red result (0 passed, 8 failed).
- T010-SCHEMA: added the branch metadata/attempt schema, replay-safe `0009`
  migration, and Docker seed registration.
- T010-QUERY: added the API projection and guarded queue/progress/error/
  completion/attempt sqlc queries.
- T010-GENERATE: `rtk sqlc generate` succeeds after the source contract changes.
- T010-HANDLER: implemented the local atomic queued transition, duplicate
  active guard, post-commit publisher, failure persistence, and queued API
  acknowledgement; T026 validation remains in place.
- T010-GREEN: formatting, sqlc generation, build, focused tests, and the full
  Go test suite completed; the two validation blockers are recorded below.
- T010-CONVERGE: README, requirements/design/tasks/status artifacts, migration,
  source queries, handler behavior, and focused tests were reviewed for
  bounded T010 scope; T003/T026 ancestry remains intact.
- T010-COMMIT: committed as `1035557ab51f183a4fe37431c07d7f5831e832e1`
  with the required co-author trailer; no push or PR was created.

## In Progress

- None.

## Blocked

- `rtk go vet ./...` and `rtk go vet ./pkg/handlers` are blocked by the
  pre-existing ignored sqlc warning
  `pkg/db/gen/models.go:1003:2: struct field _0 has json tag but is not
  exported` (T026 recorded the same unrelated vehicle-fitment warning at the
  earlier generated line 994). T010 does not edit generated output.
- Live MySQL migration execution is unavailable because
  `rtk docker version --format '{{.Server.Version}}'` cannot connect to
  `//./pipe/dockerDesktopLinuxEngine`; `docker compose config --quiet` also
  requires the absent worktree `.env`. `sqlc generate` and all Go tests pass,
  but no live DDL run is claimed.

## Backlog

- None.

## Risks

- Database/queue cross-system atomicity is impossible without the excluded
  outbox; publisher failures will be persisted as failed after the database
  commit.
- Existing legacy `csr`/`invoices` state values must remain readable.
- sqlc output is ignored and must be regenerated for each validation run.

## Next Actions

1. No further T010 implementation action remains; do not push or create a PR
   from this lane.

## Validation Evidence

- Red command:
  `rtk go test ./pkg/handlers -run 'TestZatcaOnboardingState|TestOnboardBranchZatcaPersists' -count=1`
- Red result: 0 passed, 8 failed. The existing GET query generated only 18
  destinations while the new fixture supplied 29 onboarding columns, so the
  API omitted every requested state field; the POST handler also performed no
  transaction/attempt insert and returned a config-load 500 after the scan
  mismatch.
- Green focused command:
  `rtk gofmt -w pkg/handlers/branch.go
  pkg/handlers/zatca_onboarding_state.go
  pkg/handlers/zatca_onboarding_state_test.go
  pkg/handlers/zatca_validation_test.go; rtk sqlc generate; rtk go test
  ./pkg/handlers -run 'TestZatcaOnboardingState|TestOnboardBranchZatca'
  -count=1`
- Green focused result: 16 passed.
- Build: `rtk go build ./...` — success.
- Full tests: `rtk go test ./... -count=1` — 129 passed in 6 packages.
- Formatting: `rtk gofmt -l` on all changed Go files — no output.
- Vet: blocked only by the generated `_0` warning recorded above.
- Final `rtk git diff --check` — clean.
