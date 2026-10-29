# T022 Implementation Plan

## Approach

1. Add deterministic seeded SQL and handler regressions that assert real
   tenant/branch aggregate, pending-age, retry, and provider values.
2. Replace literal state/provider fallback expressions with authoritative
   latest-outcome/current-submission selection.
3. Change branch aggregation to join durable submissions and project all state
   counts and pending/retry summaries.
4. Project latest durable attempt and pending-age/provider fields for submission
   rows.
5. Regenerate sqlc, update additive models/handlers, and preserve legacy fields,
   filters, middleware, and status mapping.
6. Run targeted red/green tests, sqlc, gofmt, build, vet, full tests, diff
   review, convergence, and commit without pushing.

## Files and Boundaries

- `pkg/db/queries/zatca_monitor.sql`: source query projections and filters.
- `pkg/model/zatca_monitor.go`: additive response fields.
- `pkg/handlers/zatca_monitor.go`: nullable generated-row mapping.
- `pkg/db/zatca_monitor_regression_test.go`: seeded SQL regressions.
- `pkg/handlers/zatca_monitor_test.go`: seeded handler regressions.
- `README.md`: backend monitor/generation note if needed.
- `specs/022-zatca-monitor-api/*`: feature artifacts.

No frontend, daemon, deployment, or outbox file is in the change set.

## Validation Matrix

| Gate | Command |
|---|---|
| Red | `rtk go test ./pkg/db ./pkg/handlers -run 'T022|ZatcaMonitor' -count=1` |
| Generation | `rtk sqlc generate` |
| Formatting | `rtk gofmt -w <changed Go files>` and `rtk gofmt -l .` |
| Targeted green | `rtk go test ./pkg/db ./pkg/handlers -run 'T022|ZatcaMonitor' -count=1` |
| Build | `rtk go build ./...` |
| Vet | `rtk go vet ./...` |
| Full tests | `rtk go test ./...` |
| Review | `rtk git diff --check` and scope/status inspection |
