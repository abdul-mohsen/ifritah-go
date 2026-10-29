# T024 Debit-Note API Contract

## Create debit note

`POST /api/v2/bill/debit`

Authentication: existing JWT middleware. Role policy: same authenticated-user
boundary as the current credit-note endpoint; the server derives tenant and
source context from the session and parent bill.

Request:

```json
{
  "bill_id": 99,
  "note": "Additional taxable charge"
}
```

Response on success: `201 Created` with the newly created debit source ID as
the JSON number.

The server atomically creates the debit source, generic pending submission,
and outbox event. NATS publication happens only after the transaction commits.

Errors:

- `400 Bad Request` for malformed JSON, missing/empty note, invalid source, or
  a parent without required branch context.
- `409 Conflict` when the parent bill already has a debit source.
- `500 Internal Server Error` for transaction, identity, submission, or outbox
  persistence failures.

No request field can override `merchant_id`, `branch_id`, `sequence_number`,
`egs_id`, document identity, outcome, or provider metadata.

## Monitor additions

Existing monitor endpoints keep their routes and fields. Each submission row
may additionally include:

```json
{
  "document_type": "debit",
  "source_id": 123,
  "sequence_number": 20260099,
  "egs_id": "egs-uuid",
  "stable_document_key": "tenant_a:7:debit:123"
}
```

Provider outcome, retry, artifact, and timestamp fields retain their existing
nullable semantics.
