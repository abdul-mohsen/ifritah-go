# T014 Data Model

## Versioned event message

| Field | JSON name | Meaning |
|---|---|---|
| Contract | `contract` | Versioned contract name |
| Version | `version` | Contract version |
| SchemaVersion | `schema_version` | Explicit envelope schema marker |
| EventID | `event_id` | Durable event identity from the outbox |
| TenantID | `tenant_id` | Tenant/database identity |
| BranchID | `branch_id` | Branch and subject partition |
| EGSID | `egs_id` | Stable branch-scoped EGS identity/fallback |
| DocumentType | `document_type` | `bill`, `credit`, `debit`, or `onboard` |
| SourceID | `source_id` | Source document ID |
| OutboxID | `outbox_id` | Committed outbox row ID |
| StableDocumentKey | `stable_document_key` | Durable logical event key |
| Attempt | `attempt` | 1-based publication attempt |
| AttemptCount | `attempt_count` | Prior failed attempts |
| AttemptID | `attempt_id` | Deterministic outbox/attempt correlation |

## Compatibility aliases

`doc_type`, `id`, `branch_id`, `db_name`, and optional `otp` stay top-level and
match the canonical values. The aliases are not a second source of identity.

## Lifecycle

The base payload is versioned at enqueue time. `outbox_id` and current attempt
metadata are finalized on committed drain because the database assigns the
outbox row ID and T007 increments attempt count on failure.
