# T014: Versioned ZATCA Event Idempotency Envelope

## Summary

Add an additive, versioned envelope to backend ZATCA events so downstream
processing can correlate tenant, branch, EGS, source document, durable outbox,
stable document, and attempt identities while deployed daemons continue reading
the legacy top-level fields.

## Requirements

1. Every publisher path emits explicit contract/version metadata.
2. The payload includes tenant, branch, EGS, document type, source ID, outbox
   ID, stable document key, and attempt metadata.
3. `doc_type`, `id`, `branch_id`, `db_name`, and `otp` remain compatible aliases.
4. The NATS message ID is deterministic and stable across retries.
5. No daemon consume/reconciliation or business-state behavior changes.
6. Focused publisher regressions prove the T007 payload is red and T014 is
   green.

## Scope

Backend publisher and outbox serialization only, plus tests, documentation,
and Spec Kit artifacts. The existing NATS subject and outbox table remain
unchanged.

## Success measures

- Versioned JSON is observed by the capturing publisher tests.
- Old decoder fields retain their prior values.
- Equal durable events use equal NATS message IDs across attempts.
- Required repository validation commands pass, with exact evidence recorded in
  `STATUS.md`.
