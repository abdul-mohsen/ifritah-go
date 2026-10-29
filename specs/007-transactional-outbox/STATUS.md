# Status

## Completed

- T007 requirements, research, design, task graph, and implementation plan
  created for the `fc084a4` backend baseline.
- Existing publication paths and T003 migration/sqlc conventions inspected.
- T007A red regressions ran and failed at the baseline:
  `TestOnboardBranchZatcaEnqueuesBeforePublication` observed one publisher
  call before commit; `TestCreditBillEnqueuesBeforePublicationFailure`
  returned 500 because the expected outbox/commit sequence was absent.
- T007B added `zatca_event_outbox`, migration `0009`, indexes, lifecycle
  states, and generated sqlc source queries.
- T007D/T007E moved bill, credit, and onboarding handlers to transactional
  enqueue, added the durable debit enqueue entry point, and added the
  post-commit claim/publish/retry drainer. Onboarding also records its durable
  enqueue attempt/error fields on `branch_zatca_config`.
- Focused green regressions pass for bill, credit, onboarding, debit enqueue,
  and durable publication failure.
- Final validation passed: `gofmt -l` was clean, `sqlc generate` completed,
  `go build ./...` passed, focused tests passed, and `go test ./...` passed.
- `go vet ./...` reproduced a pre-existing generated-code warning at
  `pkg/db/gen/models.go:994` (`_0` has a json tag but is not exported);
  `go vet -structtag=false ./...` passed without editing generated output.

## In Progress

- T007G: final diff/self-review and local commit; spec/plan/task convergence
  review found no remaining implementation gap.

## Blocked

- Live migration and end-to-end NATS verification are unavailable in this
  environment: Docker Desktop's Linux engine is not running, and `mysql` and
  `nats-server` executables are not installed. Deterministic sqlmock tests,
  sqlc parsing, build, vet workaround, and full Go tests are available.

## Backlog

- T007G/T007H: convergence, self-review, and local commit.

## Risks

- Existing deployments must apply the additive outbox migration before the
  updated handlers enqueue events.
- Generated sqlc output is ignored and may be absent until `sqlc generate`.
- Acknowledgement followed by process failure may cause a retry; stable
  JetStream idempotency is required.

## Next Actions

- Commit only the T007 backend lane; do not push or create a PR.
