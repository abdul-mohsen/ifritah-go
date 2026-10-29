# T031 signed-UBL import persistence

An authenticated backend stores T030 signed-UBL import evidence and exposes a
review-first lifecycle. Import state is separate from outbound ZATCA
submission/outbox state. The backend preserves source bytes/reference,
canonical parser data, validation/signature evidence, provenance, and an
append-only review/status history. Posting records import workflow completion
only; it does not create a purchase bill, accounting mutation, daemon event, or
provider submission.
