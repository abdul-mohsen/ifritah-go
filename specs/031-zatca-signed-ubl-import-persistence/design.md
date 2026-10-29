# T031 design

## Boundary

```text
JWT-authenticated API
  ├─ POST /zatca/import
  │    └─ BEGIN
  │        ├─ validate T030-shaped source/hash/result envelope
  │        ├─ insert zatca_import_job (tenant/status/version)
  │        ├─ insert zatca_import_evidence (bytes/reference/canonical/validation)
  │        ├─ insert validation-error rows
  │        ├─ insert initial status transition
  │        └─ COMMIT
  ├─ GET /zatca/import(s)[/:id]  (tenant-safe read)
  └─ POST /zatca/import/:id/{review,post,reject}
       └─ BEGIN; lock/check status+workflow version; append decision/transition;
          update job; COMMIT

outbound ZATCA lane (unchanged)
  └─ zatca_submission / zatca_event_outbox / daemon
```

The import lane never invokes the publisher and never writes an outbound
submission row.

## Data model

### `zatca_import_job`

Owns lifecycle identity and mutable workflow metadata:

- auto-increment `id`, authenticated `merchant_id`, optional `branch_id`;
- `status`: `review`, `approved`, `posted`, `rejected`;
- monotonic `workflow_version`;
- creator/reviewer/poster/rejector IDs and timestamps;
- review/post/rejection reference/reason fields;
- idempotency key and source hash uniqueness scoped by merchant.

### `zatca_import_evidence`

One immutable row per job containing:

- exact `original_bytes` when supplied;
- `source_reference`, `source_content_hash`, size, MIME type, original name,
  and provenance;
- canonical and parsed JSON (the T030 `UBLImport` result);
- signature and validation JSON;
- failure class/message and the complete result envelope JSON.

The row is immutable after creation. Its hash is checked against bytes at the
API boundary; source/reference fields are returned unchanged.

### `zatca_import_validation_error`

Append-only normalized copies of T030 `ValidationIssue` values, including
severity, code, field, message, optional details, actor, and timestamp.

### `zatca_import_review`

Append-only review decisions (`approve`, `request_changes`, `review`) with
reason, expected/current workflow version, actor, and timestamp.

### `zatca_import_status_history`

Append-only state transitions with from/to states, reason, transition key,
actor, workflow version, and timestamp. A unique transition key makes
idempotent retries safe.

## API contract

The create request is a JSON-compatible adapter for T030 `ImportResult`:

```json
{
  "original_bytes": "<base64 JSON []byte>",
  "source": {
    "content_hash": "...",
    "size": 123,
    "mime_type": "application/xml",
    "original_name": "supplier.xml",
    "content_reference": "upload:abc",
    "provenance": "supplier_xml"
  },
  "canonical": { "contract": "afrita.zatca.ubl-import", "version": "1" },
  "parsed": { "...": "optional alias" },
  "validation": { "well_formed": true, "signature_status": "unverified",
                  "review_required": true, "issues": [] },
  "signature": { "present": true, "status": "unverified" },
  "failure_class": "trust_unavailable",
  "failure_message": "no trusted certificate roots were supplied",
  "idempotency_key": "client-key"
}
```

The adapter also accepts `bytes` as a T030 request alias and `xml_content` as
an explicit UTF-8 convenience field. It derives missing source hash/size and
`sha256:<hash>` reference only when bytes are present. It does not claim
cryptographic verification or run parser logic.

Create returns `201` with `{id,status,workflow_version,replayed:false}`.
Exact/idempotent replays return the existing job with `replayed:true`.
Detail/list responses include evidence, validation errors, review decisions,
and transitions.

Review request:

```json
{"decision":"approve|request_changes|review","reason":"...", "expected_workflow_version":1}
```

Post request:

```json
{"post_reference":"optional internal reference", "expected_workflow_version":2}
```

Reject request:

```json
{"reason":"...", "expected_workflow_version":1}
```

All IDs and mutation queries include the authenticated `merchant_id`.

## Transition rules

| Action | Allowed current state | Result |
|---|---|---|
| review/approve | `review` | `approved` |
| review/request_changes | `review`, `approved` | state unchanged |
| post | `approved` | `posted` |
| reject | `review`, `approved` | `rejected` |

The expected workflow version is optional for compatibility but, when supplied,
must match. Updates use a status/version predicate and the transaction reloads
the row `FOR UPDATE`; stale or illegal transitions return `409`.

## Failure handling and security

- Invalid source/hash/size/contract input returns `400` before a transaction.
- Duplicate idempotency/content returns a tenant-owned replay or `409`; another
  tenant's same content is not disclosed.
- Any insert/transition failure rolls back all import-lane writes.
- Detail/list never expose another merchant's job.
- Review/post/reject do not accept merchant IDs, provider status, submission
  IDs, or outbox fields from the caller.
- Original bytes are returned as JSON base64; no path is read from the
  filesystem and no object-store fallback is assumed.

## Test strategy

- Red tests first: compile/run focused lifecycle tests against `98ae215` and
  fail because the import handler, queries, and durable tables do not exist.
- Green sqlmock tests assert exact transaction order, evidence/error/history
  writes, tenant predicates, replay behavior, guarded review/post/reject
  transitions, and absence of outbound writes.
- Run sqlc generation, formatting, build, vet, targeted package tests, and
  the full suite. Live MySQL/blob-store limitations remain documented.

## Requirement traceability

| Requirement | Design element |
|---|---|
| FR-1/FR-5 | job + status-history tables and transactional create |
| FR-2/FR-3/FR-4 | immutable evidence + normalized validation-error rows |
| FR-6/FR-7/FR-8 | review/reject/post handlers and guarded transitions |
| FR-9/FR-10 | tenant-safe queries and idempotent unique keys |
| FR-11 | separate import tables and no publisher/submission calls |
| FR-12 | migration/schema/query source with regenerated sqlc |
