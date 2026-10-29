# T003: Durable ZATCA Submission State

## Problem

The backend's ZATCA monitor reads a legacy, numeric submission row that does not
durably capture the provider outcome, retry history, or request/cleared
artifacts needed to operate and reconcile invoice submissions. As a result,
provider outcomes can be absent, collapsed into the wrong status, or impossible
to distinguish across retries and tenants.

## Goals

- Persist one stable, tenant-safe ZATCA document identity and its current
  submission state.
- Persist provider outcomes, provider codes, warnings, retry timing/count, and
  request/cleared XML or equivalent artifact metadata.
- Expose monitor queries that report the durable provider outcome rather than
  inferring it from a legacy status or response field.
- Preserve existing invoice monetary values and business truth without
  converting decimal amounts to binary floating point.
- Keep this change limited to backend persistence/contracts and monitor reads.

## In Scope

- Durable schema/migrations for ZATCA submission state, provider outcome,
  attempts, and artifacts.
- Explicit states: `pending`, `accepted`, `accepted_with_warning`, `rejected`,
  `ambiguous`, and `terminal_failure`.
- Stable document identity linked to the source bill and tenant/branch context.
- Provider status/code/message/warning data and timestamps.
- Attempt retry count and next/last attempt timestamps.
- Request XML, cleared XML, and artifact metadata/path persistence.
- SQL source queries, generated sqlc code produced by the repository pipeline,
  and monitor query contracts.
- A focused regression unit test proving the provider outcome/status is absent
  or incorrect on the base behavior and correct after this change.

## Out of Scope

- Submission daemon, worker, scheduler, or outbox behavior.
- Frontend changes or new HTTP routes.
- Provider API client changes.
- Invoice generation, signing, monetary calculations, or business posting.
- Data backfill/reconciliation of historical production rows beyond a
  compatibility-safe schema migration.

## User Scenarios & Testing

### Scenario 1: Persist a provider outcome

**Given** a source invoice has a stable document identity and a provider
response, **when** the backend records the outcome, **then** the durable
submission row exposes the provider status, provider code, warning text (when
present), and outcome timestamp for the same tenant and document.

### Scenario 2: Preserve ambiguous and terminal failures

**Given** a provider request times out or returns an unrecoverable error,
**when** the submission is recorded, **then** the monitor can distinguish
`ambiguous` from `terminal_failure` and retains the attempt/retry information
needed for follow-up.

### Scenario 3: Track artifacts across attempts

**Given** request XML and a cleared/response XML are produced, **when** the
submission attempt is recorded, **then** their paths or artifact metadata are
available without overwriting the stable document identity or previous attempt
history.

### Scenario 4: Enforce tenant boundaries

**Given** two tenants have documents with overlapping invoice numbers or
provider references, **when** monitor queries run for one tenant, **then** only
that tenant's submissions, attempts, outcomes, and artifacts are returned.

### Scenario 5: Regression on the base

**Given** the focused provider-outcome regression test is run at base commit
`9bdb0d6`, **then** it fails because the provider outcome row/status is absent
or mapped incorrectly; after the fix, the same test passes with the expected
durable row and explicit status.

## Functional Requirements

### FR-1: Stable document identity

The system MUST store a stable, non-null document identity for each ZATCA
submission and associate it with the source bill and tenant boundary. The
identity MUST remain stable across retries and MUST be unique within its
tenant.

### FR-2: Explicit outcome states

The system MUST represent exactly these operational states:
`pending`, `accepted`, `accepted_with_warning`, `rejected`, `ambiguous`, and
`terminal_failure`. Monitor reads MUST return the explicit state rather than a
legacy numeric approximation.

### FR-3: Provider outcome details

The system MUST retain provider status/code values, rejection/error codes,
warning codes/messages, and the time at which the provider outcome was
observed. Nullable fields MUST remain nullable when the provider did not
return a value.

### FR-4: Attempt and retry history

The system MUST retain attempt records or equivalent durable attempt metadata,
including attempt number, attempt start/completion timestamps, retry count, and
the next retry timestamp when another attempt is planned. A retry MUST NOT
replace the stable document identity.

### FR-5: Artifact evidence

The system MUST retain request XML and cleared/response XML paths or artifact
metadata, including artifact kind and creation time. Artifact records MUST be
associated with the relevant submission/attempt and tenant-safe document.

### FR-6: Monitor queries

The backend MUST provide source SQL queries for monitor statistics and
submission detail that read the durable explicit state and provider outcome
fields, with tenant predicates applied consistently. Queries MUST remain
compatible with the repository's sqlc generation pipeline.

### FR-7: Compatibility

Applying the migration to an existing database MUST preserve existing
submissions and invoice totals. Existing legacy status/response data MUST NOT
be silently treated as a different provider outcome when the new durable
fields are unknown.

## Non-Functional Requirements

- Tenant isolation MUST be enforced in every monitor query that returns
  document, attempt, outcome, or artifact data.
- Timestamp fields MUST use the repository's database timezone conventions and
  be queryable for ordering and retry scheduling.
- Schema constraints and indexes MUST support lookup by tenant/document,
  current state, provider reference/code, and retry time.
- SQL and generated code MUST be produced through the repository's established
  sqlc pipeline; generated output MUST NOT be hand-edited.
- The focused regression test MUST be deterministic and runnable without a live
  provider.

## Acceptance Criteria

1. A migration/source schema defines durable submission, outcome, attempt, and
   artifact data with the six explicit states and the required identity,
   provider, retry, artifact, and timestamp fields.
2. The schema enforces tenant-safe stable document identity and has indexes for
   monitor/retry lookups.
3. Source monitor SQL returns the explicit provider outcome/status and does not
   expose cross-tenant rows.
4. sqlc generation completes successfully and generated database contracts
   compile with the backend.
5. The focused regression test fails at `9bdb0d6` with evidence of missing or
   wrong provider outcome/status and passes after implementation.
6. Targeted tests, formatting, build/vet checks, and `go test ./...` pass, or
   any unresolved repository/tooling blocker is recorded precisely.
7. No daemon, frontend, outbox, or unrelated worktree changes are included.

## Assumptions

- The existing `bill.merchant_id` is the tenant boundary for invoice data, and
  `bill.branch_id` provides optional branch scope.
- The existing `zatca_submission` table is the compatibility anchor; the
  migration may extend it and add normalized child tables rather than deleting
  historical data.
- A provider outcome may be unknown after an ambiguous transport failure, so
  provider fields are nullable.
- Artifact paths are relative/application-managed references, not secrets or
  inline XML payloads by default.
- The repository's current sqlc configuration and migration directory are the
  authoritative generation pattern.

## Risks

- Existing deployments may have only the legacy numeric status and response
  columns, so migration/backfill must be additive and replay-safe.
- Hidden consumers may compile against existing generated query names; source
  query changes must preserve or deliberately update those contracts.
- Monitor endpoints currently assume a single-company deployment in parts of
  the codebase; adding explicit tenant predicates must not broaden access.
- The repository does not track generated sqlc output, so local generation and
  validation must be documented and reproducible.

## Unknowns / Open Questions

- Whether the approved plan expects a separate outcome table or outcome columns
  on the submission record. Default: keep the current outcome on the durable
  submission row and normalize attempts/artifacts as child tables.
- Whether a production deploy already has a separate migration runner. Default:
  follow the checked-in `pkg/db/migrations` convention and make the migration
  replay-safe where the repository pattern supports it.

## Observable Success Measures

- The regression test demonstrates a deterministic base failure and a passing
  explicit provider outcome after the fix.
- A monitor query for one tenant returns zero rows for another tenant's
  documents, even when invoice/provider identifiers overlap.
- A submission can be ordered by next retry and outcome timestamps without
  losing attempt or artifact evidence.
