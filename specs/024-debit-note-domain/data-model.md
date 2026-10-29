# T024 Data Model

## DebitNote

| Field | Type | Rules |
|---|---|---|
| `id` | unsigned bigint | Primary source ID; positive and immutable |
| `bill_id` | unsigned bigint | Required parent bill; foreign key |
| `merchant_id` | int | Copied from authenticated parent ownership; never request-supplied |
| `branch_id` | unsigned int | Copied from parent bill; required for ZATCA context |
| `sequence_number` | unsigned bigint nullable | Parent bill human sequence/reference |
| `egs_id` | varchar(128) | Onboarded branch EGS identity or deterministic tenant/branch fallback |
| `document_id` | varchar(128) nullable | Finalized as `debit:<id>` in the source transaction |
| `stable_document_key` | varchar(255) nullable | Finalized as `tenant:branch:debit:<id>` |
| `state` | int | `1` pending; later daemon may set `3` after accepted processing |
| `note` | text | Required KSA-10 reason |
| `created_at` | datetime | Database creation timestamp |
| `invoice_uuid` | char(36) nullable | Later provider/daemon output |
| `invoice_hash` | varchar(128) nullable | Later signed-document output |
| `invoice_qr` | mediumtext nullable | Later provider/daemon output |
| `invoice_xml_path` | varchar(500) nullable | Later signed XML reference |

Relationships: `DebitNote.bill_id -> bill.id` and
`DebitNote.branch_id -> branches.id`. A tenant may create at most one debit
source for a given parent bill in T024.

## ZatcaSubmission source identity

The existing submission row remains the status anchor and gains:

| Field | Type | Rules |
|---|---|---|
| `document_type` | varchar(16) | `bill`, `credit`, or `debit`; historical default `bill` |
| `source_id` | unsigned bigint | Source primary key; historical bill rows backfill from `bill_id` |
| `sequence_number` | unsigned bigint nullable | Source/parent sequence context |
| `egs_id` | varchar(128) nullable | Source EGS context; nullable for historical rows without branch identity |

`bill_id` remains the parent compatibility reference. Uniqueness moves from
bill-only to `(merchant_id, document_type, source_id)`.

## State transitions

```text
debit_note.state=1
  -> pending submission/outbox in one transaction
  -> later daemon/provider contract updates submission outcome
  -> debit_note.state=3 only in later daemon processing (out of scope)
```

Submission outcome states remain the existing T003 set:
`pending`, `accepted`, `accepted_with_warning`, `rejected`, `ambiguous`, and
`terminal_failure`.

## Validation rules

- `bill_id`, `merchant_id`, `branch_id`, and source IDs must be positive.
- Parent bill and debit source must share the authenticated merchant.
- `note` must be non-empty at the API boundary.
- `egs_id` must be non-empty for a newly created debit source.
- `document_id` and `stable_document_key` must be non-empty before commit.
- Stable identity must not change after outbox insertion.
