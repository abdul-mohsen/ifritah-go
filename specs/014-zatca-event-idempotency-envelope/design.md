# T014 Design

## Boundary

```text
business transaction -> durable outbox row -> envelope enrichment -> NATS/JetStream
                                      \-> legacy top-level aliases retained
```

Only the backend publisher, its outbox serialization path, focused tests,
documentation, and feature artifacts change.

## Envelope model

`Message` remains the source-compatible Go type used by existing publisher
callers, but gains versioned canonical fields:

- `contract`, `version`, and `schema_version`;
- `event_id`;
- `tenant_id`, `branch_id`, and `egs_id`;
- `document_type` and `source_id`;
- `outbox_id`;
- `stable_document_key`;
- `attempt`, `attempt_count`, and deterministic `attempt_id`;
- optional onboarding `otp`.

The original top-level fields remain emitted:

- `doc_type`;
- `id`;
- `branch_id`;
- `db_name`;
- `otp` when onboarding.

`document_id`/`id` and `document_type`/`doc_type` are intentionally matching
aliases. `source_id` is the canonical integer document identity.

## Construction and enrichment

The publisher constructs a versioned base message for direct calls and for
T007's `buildZatcaOutboxParams`. The stable key is the existing document key
(`tenant:branch:type:id`, with the existing OTP hash suffix for onboarding).

The outbox drainer decodes the stored base message and overlays committed row
context before publication:

- `outbox_id = row.id`;
- `event_id = row.event_id`;
- tenant/document/branch values from the row;
- `stable_document_key = row.idempotency_key`;
- `attempt = row.attempt_count + 1`;
- `attempt_count = row.attempt_count`;
- deterministic `attempt_id = <outbox-id>:<attempt>`.

The EGS value from the stored message is retained; if absent, the
tenant/branch fallback is applied. No database or business-state update is
performed for these fields.

## NATS identity and subject

The subject remains:

```text
zatca.<document-type>.<tenant/database>.<branch-id>
```

Every publish call supplies `nats.MsgId(stable_document_key)`. This is the
same for all attempts of one durable event and differs for distinct logical
events. No random event ID is used as the message identity.

## Compatibility and migration

The payload is additive rather than nested-only. A daemon that decodes only
the legacy five fields still sees the same values and can derive the same
subject. New daemons can read the versioned fields and attempt context. No
consumer/reconciliation changes are made in this repository.

## Failure handling

Envelope construction and JSON serialization errors fail publication before
NATS is called. Existing outbox failure recording remains unchanged. A NATS
acknowledgement still precedes the existing durable published-state update.

## Test strategy

- Add a capturing JetStream fake that records payload bytes and applies
  `nats.PubOpt` values to inspect `Nats-Msg-Id`.
- Prove the base commit is red because it omits versioned identity and
  publishes without a deterministic message ID.
- Assert the final outbox envelope includes row ID and attempt metadata while
  retaining all legacy aliases.
- Publish the same row twice with different attempt counts and assert the
  payload attempt metadata changes but the message identity remains stable.
- Run existing T007 tests unchanged, then run generation, formatting, build,
  vet, targeted publisher tests, and the full Go suite.

## Rollout and rollback

Deploying the backend is additive for the daemon because old fields remain.
The existing NATS subject and durable outbox state are unchanged. Rollback to
T007 remains possible because the daemon still receives the legacy aliases;
the new fields are ignored by old decoders.

## Requirement traceability

| Requirement | Design element |
|---|---|
| FR-1 | Contract/version/schema-version fields on every outbound message |
| FR-2 | Canonical tenant/branch/EGS/document/source/outbox/stable-key fields |
| FR-3 | Attempt number/count and deterministic attempt ID |
| FR-4 | Stable key passed as `nats.MsgId` |
| FR-5 | Top-level legacy aliases and unchanged subject |
| FR-6 | Publisher-only enrichment; no daemon or business-state edits |
| FR-7 | Capturing publisher regressions and full validation gates |
