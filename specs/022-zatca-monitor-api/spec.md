# T022: Real ZATCA Monitor Aggregates

## Summary

Make the existing ZATCA monitor accurately describe tenant and branch
submissions from durable records, including complete state counts, pending age,
retry evidence, and provider details, without synthesized fallback values.

## User Value

Administrators can trust the monitor to investigate provider outcomes and
pending/retry work for their own tenant without confusing configured branches,
legacy defaults, or missing provider evidence with real submissions.

## Scope

### In scope

- Existing monitor statistics, branch, and submission responses.
- Durable-row aggregation and provider/retry projections.
- Tenant-safe seeded SQL/handler regression tests.
- Backend documentation and generated SQL validation.

### Out of scope

- Frontend, daemon, deployment, outbox, provider-client, route, or permission
  changes.

## Requirements

1. Count only durable rows belonging to the authenticated tenant.
2. Provide accepted, accepted-with-warning, rejected, pending, ambiguous, and
   terminal-failure counts for tenant and branch aggregates.
3. Include real branch totals and omit branches without durable submissions.
4. Include nullable pending age and durable retry evidence.
5. Include provider code/reference/warning evidence from the authoritative
   durable outcome.
6. Preserve existing routes, filters, middleware, and response fields.
7. Keep source SQL generated through sqlc and add regressions that require
   seeded rows.

## Success Criteria

- Seeded tenant and branch rows produce exact expected counts with no
  cross-tenant rows.
- Seeded pending rows produce the expected pending timestamp/age and retry
  state.
- Seeded provider outcomes produce matching provider details; missing optional
  values remain null.
- Focused tests, sqlc generation, formatting, build, vet, and the full Go test
  suite pass.
