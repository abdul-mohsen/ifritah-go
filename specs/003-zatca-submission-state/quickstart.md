# T003 Validation Quickstart

Run from the repository root.

## Prerequisites

- Go toolchain from `go.mod`.
- `sqlc` available on `PATH`.
- Dependencies downloaded by the normal Go toolchain.

## Generate database contracts

```powershell
rtk sqlc generate
```

Generated files under `pkg/db/gen` are ignored build artifacts; do not edit
them by hand.

## Focused regression

```powershell
rtk go test ./pkg/db -run '^TestZatcaMonitorReturnsProviderOutcome$' -count=1
```

The test must fail at base `9bdb0d6` with a scan mismatch because the legacy
monitor query has no provider outcome columns, then pass after generation and
the T003 source changes.

## Full validation

```powershell
rtk gofmt -l .
rtk go build ./...
rtk go vet -structtag=false ./...
rtk go test ./...
```

The monitor query contract is documented in
[`contracts/zatca-monitor.md`](contracts/zatca-monitor.md), and the table
relationships are documented in [`data-model.md`](data-model.md).
