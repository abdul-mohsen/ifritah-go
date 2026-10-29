# ZATCA Event Envelope Contract

The backend publishes JSON on the existing subject:

```text
zatca.<document_type>.<tenant/database>.<branch_id>
```

The additive envelope includes:

```json
{
  "contract": "afrita.zatca.document-event",
  "version": "1",
  "schema_version": "1",
  "event_id": "evt-...",
  "tenant_id": "test_tenant",
  "branch_id": 7,
  "egs_id": "test_tenant:branch:7",
  "document_type": "bill",
  "source_id": 42,
  "outbox_id": 123,
  "stable_document_key": "test_tenant:7:bill:42",
  "attempt": 1,
  "attempt_count": 0,
  "attempt_id": "123:1",
  "doc_type": "bill",
  "id": 42,
  "db_name": "test_tenant"
}
```

The `doc_type`, `id`, `branch_id`, `db_name`, and optional `otp` fields remain
top-level for existing daemon decoders. JetStream receives
`Nats-Msg-Id: test_tenant:7:bill:42` for every attempt of that durable event.
