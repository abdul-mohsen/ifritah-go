# T022 Monitor Contract

Existing routes remain unchanged:

- `GET /api/v2/zatca/monitor/stats`
- `GET /api/v2/zatca/monitor/branches`
- `GET /api/v2/zatca/monitor/submissions`

Existing JSON fields remain compatible. Additive fields are:

## Stats

- `oldest_pending_at`: nullable RFC3339 timestamp
- `pending_age_seconds`: nullable integer

## Branch rows

- `total_submitted`, `accepted`, `accepted_with_warning`, `warnings`,
  `rejected`, `pending`, `ambiguous`, `terminal_failure`
- `oldest_pending_at`, `pending_age_seconds`
- `pending_retry_count`, `pending_next_retry_at`

## Submission rows

- `attempt_number`, `retry_state`
- `retry_error_code`, `retry_error_message`
- `pending_since`, `pending_age_seconds`

Provider fields are returned from the latest durable outcome when one exists,
otherwise from the current durable submission row. Optional evidence is
serialized as `null`; it is never filled with a synthetic value.
