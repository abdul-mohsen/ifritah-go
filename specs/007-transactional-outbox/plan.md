# T007 Implementation Plan

## Summary

Add a durable ZATCA outbox with an additive migration and generated sqlc
queries, move bill/credit/onboarding publication to transactional enqueue
paths, retain a durable debit enqueue API, and add a post-commit publisher
drainer with retry/failure state.

## Phases

1. Run focused sqlmock/handler red tests at `fc084a4`.
2. Add outbox table, migration, schema source, and sqlc queries.
3. Add message serialization/enqueue helpers and update handlers.
4. Add drain/claim/retry lifecycle and main startup/shutdown integration.
5. Add green regressions, regenerate sqlc, format, build, vet, and run all
   tests.
6. Update README/status, converge, self-review, and commit only this lane.

## Decisions

- Use a dedicated outbox table; do not overload provider submission state.
- Persist the exact event payload and idempotency key for safe retries.
- Publish outside database locks after a short conditional claim.
- Keep existing auth, tenant, store, branch, event, and subject semantics.
- Do not add a debit route because the baseline has only the existing
  `SubmitDebit` publisher entry point.

## Validation Matrix

| Phase | Command/check |
|---|---|
| Red baseline | `rtk go test ./pkg/handlers -run 'Test(AddBill|CreditBill|OnboardBranchZatca).*Outbox' -count=1` |
| Red durable failure | `rtk go test ./pkg/handlers -run 'TestZatca.*PublicationFailure' -count=1` |
| Generation | `rtk sqlc generate` |
| Formatting | `rtk gofmt -w <changed Go files>` and `rtk gofmt -l <changed Go files>` |
| Targeted green | same focused commands as red |
| Build | `rtk go build ./...` |
| Vet | `rtk go vet ./...` |
| Full tests | `rtk go test ./...` |
| Scope review | `rtk git diff --check`, status, and diff inspection |
