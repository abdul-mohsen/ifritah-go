# T024 Implementation Plan

## Approach

1. Define the additive source/submission schema and sqlc contracts.
2. Add the authenticated debit transaction and durable publisher context.
3. Extend the existing envelope and monitor response/query additively.
4. Prove the missing flow with focused red regressions, then regenerate SQL
   and run the complete validation gates.

## Key decisions

- The parent bill supplies human sequence and branch; no separate debit
  sequence is allocated.
- The onboarded `csr_computer_number` is preferred for EGS identity, with the
  existing tenant/branch fallback.
- Generic submissions keep `bill_id` as the parent compatibility link and add
  document kind/source identity so debit and credit do not collide.
- The API registers `pending` status in the same transaction but does not
  process XML or provider responses.

## Work products

- `pkg/db/migrations/0011_zatca_debit_note.sql`
- `pkg/db/migrations/0012_zatca_submission_sources.sql`
- `pkg/db/schema/schema.sql`
- `pkg/db/queries/debit.sql`
- `pkg/db/queries/zatca_monitor.sql`
- `pkg/handlers/debit.go`, `pub.go`, `zatca_outbox.go`,
  `zatca_envelope.go`, `zatca_monitor.go`
- `pkg/model/zatca_monitor.go`, `main.go`, `README.md`
- focused regression tests and generated sqlc output (ignored pipeline files)

## Validation

Run the red baseline before source implementation, then:

```powershell
rtk sqlc generate
rtk gofmt -w pkg/db/queries pkg/db/schema pkg/db/migrations pkg/handlers pkg/model
rtk go build .
rtk go vet ./...
rtk go test ./pkg/handlers ./pkg/db -run 'Debit|Zatca' -count=1
rtk go test ./... -count=1
rtk git diff --check
```

The final report must state that live MySQL migration/locking/provider
validation was not available.
