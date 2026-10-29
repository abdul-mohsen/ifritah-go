# T003 Implementation Plan

## Summary

Add an additive, tenant-safe ZATCA persistence model around the existing
`zatca_submission` record, expose explicit provider outcomes through source
monitor queries, regenerate sqlc contracts, update the existing monitor
handler/model, and validate with the pre-added red regression test.

## Implementation Phases

1. **Schema/migration**: extend the schema dump; add replay-safe current-state
   fields and outcome/attempt/artifact tables; add indexes and seed registration.
2. **Queries**: update all monitor queries with merchant scope and explicit
   outcome/provider/retry/artifact projections while preserving legacy fallback.
3. **Generation**: run the repository sqlc pipeline and use generated types only.
4. **Contracts**: update monitor models/handler to pass tenant scope and expose
   explicit provider state/details.
5. **Validation**: run focused regression, formatting, build, vet, and full tests;
   review artifacts/diff and commit without push.

## Design Decisions

- Keep numeric `status` for compatibility; use checked text `outcome_state` for
  the six operational states.
- Scope every document query through `bill.merchant_id`, the current tenant key.
- Keep current state on the submission for fast monitor counts and append
  provider observations in `zatca_submission_outcome`.
- Preserve attempts/artifacts as child history; use paths/metadata rather than
  changing invoice amounts or requiring daemon/frontend changes.

## Risks and Mitigations

- **Ignored generated output**: run `sqlc generate` locally and rely on CI's
  existing generation step; never hand-edit generated files.
- **Legacy rows**: additive migration/backfill keeps numeric status/response and
  defaults missing explicit outcome to `pending`.
- **Tenant leakage**: require merchant id in stats/branches/submission queries
  and pass it from the authenticated session.
- **Migration parser compatibility**: validate against the repository's sqlc
  parser and existing MySQL migration style.

## Validation Matrix

| Phase | Command/check |
|---|---|
| Red baseline | `rtk proxy go test ./pkg/db -run '^TestZatcaMonitorReturnsProviderOutcome$' -count=1` |
| Generation | `rtk sqlc generate` |
| Formatting | `rtk gofmt -w` for changed Go files; `rtk gofmt -l .` |
| Targeted regression | `rtk go test ./pkg/db -run '^TestZatcaMonitorReturnsProviderOutcome$' -count=1` |
| Build | `rtk go build ./...` |
| Vet | `rtk go vet -structtag=false ./...` |
| Full tests | `rtk go test ./...` |
| Review | `rtk git diff --check`, inspect diff/status |
