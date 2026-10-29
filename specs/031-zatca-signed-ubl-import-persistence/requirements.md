# T031 — Durable signed-UBL import persistence

## Problem

The T030 signed-UBL service returns exact source bytes, provenance, canonical
UBL data, signature evidence, validation state, and classified failures, but
the backend has no durable intake record or authenticated workflow around that
result. A process restart therefore loses the source evidence and reviewers
cannot approve, post, or reject an import without mutating outbound ZATCA
submission state.

## Goals

- Persist every authenticated signed-UBL import as a tenant-scoped job.
- Preserve the exact original bytes when supplied, their content reference,
  hash, size, MIME type, original name, and provenance.
- Preserve the T030 canonical/parsed result, signature evidence, validation
  state, failure class/message, and validation issues without reinterpreting
  them as an outbound submission outcome.
- Persist an append-only lifecycle/status history and review decisions.
- Expose authenticated create, list, detail, review, post, and reject APIs.
- Keep the import lane explicitly separate from `zatca_submission`,
  `zatca_event_outbox`, daemon processing, and frontend behavior.
- Make duplicate content/idempotent requests safe and tenant-isolated.

## In scope

- Additive MySQL migration and checked-in schema source for import jobs,
  immutable evidence, validation errors, review decisions, and status history.
- sqlc query sources for create/read/list and guarded lifecycle transitions.
- JWT-protected Gin routes under the existing API group.
- Read-only alignment with the T030 `ImportResult`, `XMLSourceEvidence`,
  `UBLImport`, `ValidationState`, `ValidationIssue`, and failure-class
  contract. The backend does not embed or call the T030 parser.
- Focused sqlmock lifecycle regressions, documentation, and Spec Kit
  artifacts.

## Out of scope

- Frontend UI, daemon changes, NATS/outbox events, `zatca_submission` rows,
  provider submission, signing, XML generation, inventory/accounting, and
  automatic purchase-bill creation.
- Reimplementing T030 parsing, XML safety, cryptographic verification, or
  schema validation.
- Live MySQL/object-storage integration in the focused test suite.

## User stories

### US1 — Durable import evidence

As an authenticated tenant user, I can submit a T030-shaped import result and
receive a durable job ID. Restarting the process does not lose the original
bytes/reference, canonical data, provenance, signature evidence, or validation
errors.

### US2 — Review workflow

As an authenticated reviewer, I can inspect a tenant-owned import and record an
append-only review decision. The workflow version and status history make the
decision auditable and reject stale/concurrent transitions.

### US3 — Explicit post/reject decisions

As an authenticated reviewer, I can post an approved import or reject a
reviewable import. These transitions persist actor, time, reason/reference, and
status history without creating or changing an outbound ZATCA submission.

### US4 — Tenant-safe retrieval

As an authenticated tenant user, I can list and retrieve only imports belonging
to my tenant. A job ID from another tenant is indistinguishable from not found.

## Functional requirements

- **FR-1 Job identity and scope:** Each accepted request MUST create one
  tenant-scoped import job with a positive stable ID, creator, status, and
  workflow version. The authenticated identity, not a caller-supplied tenant,
  supplies the tenant boundary.
- **FR-2 Source preservation:** The backend MUST retain exact original bytes
  when present, plus source reference, SHA-256, byte size, MIME type, original
  name, and provenance. If bytes are absent, a non-empty source reference and
  verified content hash MUST still be retained.
- **FR-3 Canonical/evidence preservation:** The backend MUST retain the
  T030-shaped canonical/parsed JSON, signature evidence, validation state,
  failure class/message, and unknown/unsupported evidence without silently
  converting it into a provider outcome.
- **FR-4 Validation issues:** Every supplied T030 validation issue MUST be
  queryable as a durable tenant-scoped validation-error row with severity,
  code, optional field, message, and JSON details.
- **FR-5 Status history:** Job status transitions MUST be append-only,
  tenant-scoped, actor-attributed, and protected by the current workflow
  version. The initial import status is `review`.
- **FR-6 Review:** `POST /api/v2/zatca/import/:id/review` MUST persist a
  review decision. `approve` transitions `review` to `approved`; a
  `request_changes`/`review` decision records review evidence and keeps the
  job in `review`.
- **FR-7 Post:** `POST /api/v2/zatca/import/:id/post` MUST transition only an
  approved job to `posted`, persist actor/time and an optional post reference,
  and MUST NOT write `zatca_submission`, `zatca_event_outbox`, or call NATS.
- **FR-8 Reject:** `POST /api/v2/zatca/import/:id/reject` MUST require a
  non-empty reason, transition a non-posted job to `rejected`, and persist
  actor/time/reason and status history.
- **FR-9 Retrieval APIs:** Authenticated list/detail responses MUST expose
  status/version, source/evidence, validation errors, review decisions, and
  status history while retaining existing response/error conventions.
- **FR-10 Idempotency and duplicates:** Replaying an idempotency key or exact
  tenant/content hash MUST not create a second job. A cross-tenant same hash
  MUST remain allowed and invisible across tenants.
- **FR-11 Scope separation:** Import lifecycle state MUST use its own tables and
  status vocabulary. It MUST NOT reuse outbound submission state or alter
  existing submission/outbox/daemon behavior.
- **FR-12 Generated SQL:** All new database access MUST be represented in
  source schema/migration/sqlc files; generated sqlc output MUST be regenerated
  and not hand-edited.

## Non-functional requirements

- MySQL 8 compatible, parameterized SQL with tenant predicates on every read or
  mutation.
- Request limits and JSON/hash consistency checks prevent storing contradictory
  source evidence.
- Lifecycle transitions are atomic and safe under concurrent requests.
- Existing ZATCA submission/outbox/sequence/debit behavior remains unchanged.
- Focused tests run without live MySQL, object storage, NATS, daemon, or
  frontend services.

## Acceptance criteria

1. Red lifecycle regressions fail on `98ae215` because no durable signed-UBL
   import job/evidence/lifecycle record or API exists.
2. Green regressions prove create persistence, exact source/canonical/error
   evidence, tenant-safe detail, review approval, post, reject, and status
   history.
3. Post/reject/review tests prove no outbound submission or outbox write is
   performed.
4. Migration and schema define import job, evidence, validation-error,
   review-decision, and status-history persistence with tenant indexes and
   uniqueness for idempotent content/key.
5. `sqlc generate`, `gofmt`, build, vet, targeted tests, and `go test ./...`
   are run; unavailable live MySQL/object storage limitations are reported.
6. Only backend/import artifacts, relevant README/spec files, and generated
   ignored sqlc output change. No frontend or daemon files change.
7. One isolated local commit is created; nothing is pushed and no PR is
   created.

## Assumptions, constraints, risks, and unknowns

- The authenticated session ID is the existing ZATCA tenant boundary
  (`merchant_id`) used by submission/outbox queries; no caller-supplied tenant
  is trusted.
- T030 is in a separate repository and is an API contract only for this task;
  the backend accepts its JSON-shaped result and does not import its Go
  package.
- Original bytes may be supplied as JSON base64 (`[]byte` encoding) or omitted
  when an immutable external reference and verified hash are available.
- The backend cannot prove object-store durability or MySQL lock/isolation
  behavior with sqlmock; these remain rollout risks.
- “Post” records import workflow completion only. It intentionally does not
  create a purchase bill, stock movement, accounting entry, or outbound ZATCA
  submission.
- Existing role middleware is not changed; routes remain JWT-protected and
  lifecycle handlers enforce tenant ownership and status/version guards.

## Observable success measures

- A committed create request leaves exactly one import job, one evidence row,
  zero-or-more validation-error rows, and one initial status-history row.
- A detail response after restart contains the same source hash/bytes/reference,
  canonical parsed data, provenance, validation issues, and failure evidence.
- Review/post/reject create only import-lane rows and transitions; no
  `zatca_submission` or `zatca_event_outbox` write is observed.
- A second tenant cannot retrieve or mutate the first tenant's job.
