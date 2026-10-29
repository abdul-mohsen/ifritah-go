# T016 Quickstart

Run from the T016 worktree:

```powershell
rtk sqlc generate
rtk gofmt -w pkg/handlers/bill.go pkg/handlers/zatca_sequence_regression_test.go
rtk go test ./pkg/handlers -run 'Sequence|Chain|Outbox'
rtk go build ./...
rtk go vet ./...
rtk go test ./...
```

Expected outcomes:

1. Focused allocator tests show branch-local monotonic values and no global
   `MAX(sequence_number)` query.
2. Chain tests reject owner-mismatched commit/release and preserve state on
   rollback.
3. Existing T014 outbox/envelope tests remain green.
4. Full build, vet, and test suites pass.

These commands do not exercise live MySQL row locks or a ZATCA daemon. Those
remain an explicit pre-production validation requirement.
