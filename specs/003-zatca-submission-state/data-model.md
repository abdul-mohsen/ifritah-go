# T003 Data Model

## ZATCA Submission

Existing table extended in place.

| Field | Meaning | Rules |
|---|---|---|
| `id` | Durable submission row id | Primary key |
| `bill_id` | Source invoice | Existing foreign key; one submission per bill |
| `merchant_id` | Tenant identity copied from the source bill | Required; part of the stable document uniqueness key |
| `document_id` | Stable document identity | Non-null; backfilled from invoice UUID or `bill:<id>` |
| `outcome_state` | Current explicit state | `pending`, `accepted`, `accepted_with_warning`, `rejected`, `ambiguous`, `terminal_failure` |
| `provider_code` | Current provider code/status | Nullable |
| `provider_warning_code` | Current warning code | Nullable |
| `provider_warning_message` | Current warning text | Nullable |
| `provider_reference` | Provider submission/reference id | Nullable |
| `outcome_at` | Current outcome observation time | Nullable |
| `retry_count` | Current retry count | Non-negative |
| `last_attempt_at` | Latest attempt time | Nullable |
| `next_retry_at` | Scheduled retry time | Nullable |
| `request_xml_path` | Current request XML reference | Nullable |
| `cleared_xml_path` | Current cleared XML reference | Nullable |
| `created_at`, `updated_at` | Row lifecycle timestamps | Existing timestamp behavior |

The legacy numeric `status`, rejection fields, submitted/cleared timestamps,
response XML, and invoice monetary fields remain for compatibility.

## Submission Outcome

Append-only provider observations linked to a submission and optional attempt.
Stores explicit state, provider code/reference, warning/rejection detail, and
observed/created timestamps. The latest row is selected by observation time
then id.

## Submission Attempt

One provider request try. Stores submission id, monotonically increasing
attempt number, explicit attempt state, provider/error identifiers, retry
count/next retry time, request/cleared XML paths, start/completion times, and
row timestamps. `(submission_id, attempt_number)` is unique.

## Submission Artifact

Evidence metadata linked to a submission and optional attempt. Artifact kinds
are `request_xml`, `cleared_xml`, `response_xml`, and `other`. Each row stores a
path/reference, content type, SHA-256, optional JSON metadata, and created time.

## Relationships

```text
bill (tenant via merchant_id)
  1 ── 0..1 zatca_submission
             1 ── * zatca_submission_outcome
             1 ── * zatca_submission_attempt
             1 ── * zatca_submission_artifact
             attempt 1 ── * artifact/outcome (optional link)
```

Tenant isolation is enforced by the copied submission tenant key plus the
source `bill` relationship and explicit `merchant_id` predicates in monitor
queries.
