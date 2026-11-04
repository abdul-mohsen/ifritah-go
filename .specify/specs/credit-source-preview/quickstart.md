# Quickstart: Bill Credit-Source Preview

## Focused validation

From `C:\ssda\chatGPT\ifritah\backend.git\worktree\backend-b6-notes-2`:

```powershell
rtk go test ./pkg/handlers -run 'TestGetBillCreditSource'
```

Expected: handler tests pass for a bill with no prior credits, partial credit history, fully credited bills, and missing/unverifiable source data or status.

## Package validation

```powershell
rtk go test ./pkg/handlers
```

Expected: existing handler tests and the new endpoint tests pass with sqlmock; no live database is required.

## Format check

```powershell
rtk gofmt -w pkg/handlers/credit.go pkg/handlers/credit_source_test.go main.go
```

Expected: changed Go source and test files are formatted. Review the diff afterward to ensure no unrelated files changed.

See [the API contract](contracts/credit-source.md) for the response and preview semantics.
