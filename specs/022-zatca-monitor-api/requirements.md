# T022: Real ZATCA Monitor Aggregates

## Problem

The ZATCA monitor can expose values that are not faithful to the authenticated
tenant's durable submission records. Branch rows are not complete submission
aggregates, provider and retry evidence is incomplete, and fallback values can
make an empty or incomplete durable record appear to have a real operational
state. Operators need the monitor to describe the submissions that actually
exist for the selected tenant and branch.

## Goals

- Report tenant-scoped and branch-scoped aggregates from durable submission
  records.
- Report accepted, accepted-with-warning, rejected, pending, ambiguous, and
  terminal-failure counts.
- Report how long pending submissions have been waiting and the durable retry
  state/timing that operators need for follow-up.
- Report provider outcome details without inventing values when the provider did
  not return them.
- Preserve existing monitor routes and response fields while adding compatible
  fields.

## In Scope

- ZATCA monitor source queries and their generated database contracts.
- Monitor response models and handlers.
- Focused seeded SQL and handler regression tests.
- Backend documentation and feature artifacts for this change.

## Out of Scope

- Frontend, daemon/worker, deployment, provider-client, and outbox changes.
- New routes or permission changes.
- Changes to invoice totals, bill posting, or submission persistence writers.
- Reconciliation or backfill of historical data beyond using the durable rows
  already defined by T003.

## User Scenarios

### Scenario 1: Tenant statistics

Given durable submissions for multiple tenants, when an authenticated admin
requests monitor statistics, then only the current tenant's submissions are
counted and each of the six explicit states has the correct count.

### Scenario 2: Branch operations view

Given durable submissions across branches, when an authenticated admin requests
the branch monitor, then each returned branch has counts derived from its
tenant-scoped submissions, pending age derived from pending durable timestamps,
and compatible existing daily/weekly metrics.

### Scenario 3: Submission investigation

Given a submission with a latest durable provider outcome and retry attempt,
when the submission monitor is requested, then the response includes the
provider code/reference/warning details, retry state/count/timestamps, and
pending age or outcome timestamps from those rows.

### Scenario 4: Missing optional evidence

Given a durable submission without a provider warning, provider reference, or
retry attempt, when it is returned, then those optional response fields remain
null rather than being copied from unrelated or fabricated defaults.

### Scenario 5: Tenant isolation

Given matching invoice/provider identifiers in two tenants, when either tenant
requests monitor statistics, branches, or submissions, then no row or aggregate
from the other tenant is returned.

## Functional Requirements

### FR-1: Durable tenant aggregates

The monitor MUST count only durable submission rows linked to bills belonging to
the authenticated tenant. The total and every state count MUST equal the
matching durable rows; empty or unrelated rows MUST NOT create counts.

### FR-2: Complete explicit-state counts

The monitor MUST expose counts for `accepted`,
`accepted_with_warning` (including the existing `warnings` compatibility field),
`rejected`, `pending`, `ambiguous`, and `terminal_failure` at the tenant and
branch levels where aggregates are returned.

### FR-3: Branch aggregation

The branch monitor MUST return only branches with in-scope durable submission
rows and MUST provide actual branch totals and state counts. Existing branch
status, certificate, today, weekly success-rate, and last-submission fields
MUST remain compatible.

### FR-4: Pending age

For pending durable submissions, the monitor MUST expose the oldest pending
timestamp and/or age in a clearly documented form. When no pending row exists,
pending-age fields MUST be null or otherwise explicitly empty, not a fabricated
duration.

### FR-5: Retry evidence

Submission responses MUST expose durable retry count and timing fields already
supported by the API and MUST expose the latest durable attempt/retry state when
one exists. Missing attempt history MUST remain null/empty.

### FR-6: Provider evidence

Submission responses MUST expose provider code, warning code/message, and
provider reference from the current durable outcome or current durable
submission row. An outcome row with a null optional value MUST NOT be filled from
an older or unrelated value.

### FR-7: API compatibility

The three existing monitor routes, query parameters, authorization behavior,
legacy numeric status field, and existing JSON fields MUST continue to work.
New fields MUST be additive.

### FR-8: Generated SQL pipeline

All source query changes MUST remain compatible with the repository's sqlc
generation pipeline. Generated output MUST be regenerated, not hand-edited.

### FR-9: Regression protection

Focused seeded SQL and handler tests MUST fail against the current T003 monitor
behavior when real durable aggregate/provider/retry rows are not projected, and
MUST pass after the T022 implementation.

## Non-Functional Requirements

- Every monitor query that returns submission data or aggregates MUST enforce the
  authenticated tenant predicate through both the bill relationship and durable
  submission tenant key.
- No monitor query or handler may synthesize a pending state, provider detail,
  retry state, branch row, or aggregate in place of a missing durable row.
- Existing error responses and admin-only route middleware MUST remain intact.
- The implementation MUST not modify frontend, daemon, deployment, or outbox
  files.

## Acceptance Criteria

1. Seeded SQL regression tests demonstrate the six state counts and tenant
   filtering from durable rows.
2. Seeded branch regression tests demonstrate branch totals/counts and pending
   age from durable rows, while a branch with no submission rows is absent.
3. Seeded handler regression tests demonstrate provider and retry fields are
   returned from durable query rows and optional values are not fabricated.
4. All three monitor routes preserve existing paths and legacy fields.
5. Source queries contain no literal pending/provider/retry fallback used to
   manufacture a row's state or details.
6. sqlc generation, formatting, build, vet, targeted tests, and `go test ./...`
   pass.
7. The final commit contains only T022 backend/docs/test changes, has the
   required co-author trailer, and is not pushed.

## Assumptions

- `bill.merchant_id` is the authoritative tenant boundary and
  `zatca_submission.merchant_id` is the durable tenant consistency key.
- `zatca_submission.outcome_state` is the current durable state; the latest
  `zatca_submission_outcome` is the most recent durable provider observation.
- `zatca_submission_attempt` ordered by attempt number/time is the durable
  source for latest retry state.
- Existing nullable fields should serialize as null to preserve the distinction
  between missing provider evidence and an actual empty value.
- Pending age is measured from the durable submission timestamp, falling back
  only to the same row's durable creation timestamp when submission time is
  absent.

## Risks

- sqlc's MySQL type inference for nullable aggregate expressions may require
  explicit handler handling.
- Existing consumers may rely on the current `warnings` alias and legacy status
  mapping.
- Existing databases may contain legacy rows with empty explicit state; those
  rows must not cause fabricated provider details or counts.

## Unknowns Resolved by Research

- The current T003 query uses literal `'pending'` fallback expressions and
  branch existence based on bills rather than durable submissions.
- The schema has durable current submission, outcome, and attempt tables with
  nullable provider/retry evidence.
- The existing handler uses a concrete sqlc query object, so seeded handler
  tests can use sqlmock plus the generated query contract without introducing a
  new interface.

## Observable Success Measures

- For seeded tenant data, every returned count equals the seeded durable row
  count and never includes another tenant.
- For seeded pending data, pending age and retry fields match the durable
  timestamps/state.
- For seeded provider data, provider values in JSON match the latest durable
  outcome and remain null when absent.
