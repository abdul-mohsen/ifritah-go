# T022 Validation Quickstart

From the repository worktree:

```powershell
rtk sqlc generate
rtk gofmt -w pkg/db/zatca_monitor_regression_test.go pkg/db/zatca_monitor_t022_test.go pkg/handlers/zatca_monitor.go pkg/handlers/zatca_monitor_t022_test.go pkg/model/zatca_monitor.go
rtk go test ./pkg/db ./pkg/handlers -run 'T022|ZatcaMonitor' -count=1
rtk go build ./...
rtk go vet ./...
rtk go test ./...
```

The focused tests use seeded `go-sqlmock` rows and verify:

1. tenant state totals and pending age;
2. branch state totals, pending age, retry count, and tenant argument;
3. provider details, latest attempt state/error, pending age, and null
   optional provider values in handler JSON.

See [the API contract](contracts/zatca-monitor.md) and
[the data model](data-model.md) for the expected fields.
