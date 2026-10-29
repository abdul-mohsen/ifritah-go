# T024 Debit Event Contract

The event uses the existing `afrita.zatca.document-event` envelope and subject
`zatca.debit.<tenant>.<branch>`.

Required stable values:

| Field | Value |
|---|---|
| `doc_type` / `document_type` | `debit` |
| `id` / `source_id` / `document_id` | debit source ID |
| `tenant_id` / `db_name` | selected tenant identity |
| `branch_id` | parent bill branch |
| `egs_id` | persisted branch EGS identity/fallback |
| `stable_document_key` / `idempotency_key` | `tenant:branch:debit:source_id` |
| `event_id` | deterministic hash of stable key |
| `sequence_number` | parent bill sequence when available |

`outbox_id`, `attempt`, `attempt_count`, and `attempt_id` retain the existing
drainer semantics. Retrying one committed outbox row must not mint another
stable key or source ID.

The daemon may ignore the additive `sequence_number` field until its own debit
processing contract is updated; T024 does not modify daemon code.
