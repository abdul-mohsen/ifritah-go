# T014 Research

## Evidence from the base commit

- `pkg/handlers/pub.go` defines the legacy `Message` with only
  `doc_type`, `id`, `branch_id`, `db_name`, and optional `otp`.
- `ZatcaPublisher.publish` marshals that message and calls JetStream
  `Publish(subject, data)` without a `nats.MsgId` option.
- `pkg/handlers/zatca_outbox.go` creates the payload before the database insert,
  so the auto-increment outbox row ID is unavailable while the request
  transaction is open.
- The outbox drainer already has the committed row, including `id`,
  `event_id`, `tenant_id`, `document_type`, `document_id`, `branch_id`,
  `idempotency_key`, and `attempt_count`, before it publishes.
- The existing drainer passes `idempotency_key` to `publishPayload`, and that
  path already supplies it as a JetStream message ID. The payload itself,
  however, remains legacy-only.

## Historical contract evidence

Commit `d52c8c4` (`feat: publish versioned ZATCA event contracts`) established
the repository's earlier migration convention:

- explicit `contract` and `version` fields;
- canonical tenant/document fields alongside the legacy aliases;
- stable document idempotency keys;
- the legacy `OnboadBranch` spelling retained as a wrapper;
- `nats.MsgId` populated from the stable idempotency key.

T014 reuses those compatibility conventions and adds the outbox/attempt
context that T007 introduced. It does not import the historical commit because
T007's current outbox APIs and schema are not present in that commit.

## EGS identity evidence

The checked-in schema documents `branch_zatca_config.csr_computer_number` as
the EGS UUID and `csr_tin` as the EGS serial. The current publisher does not
load branch configuration, and the outbox row does not store either value.
Adding a branch-config read or changing outbox persistence would widen this
publisher-only task and alter handler SQL expectations. T014 therefore uses a
deterministic branch-scoped EGS identity fallback (`<tenant>:branch:<branch>`)
in the envelope, while allowing an explicit EGS value on the message type for
future/configured callers. This keeps the envelope non-empty and stable
without changing business state or secrets.

## Resolved design questions

### Outbox ID

The envelope is finalized at committed publication time. The stored outbox
payload is already versioned and carries the stable event identity; the
drainer enriches it with the actual auto-increment row ID before sending.
This avoids a second write inside every business transaction and preserves
T007's duplicate-safe insert.

### Attempt metadata

The first publication uses `attempt_count + 1`; a failed outbox row is
incremented by T007 when failure is recorded, so the next drain naturally
gets the next attempt number. The envelope also includes a deterministic
attempt ID derived from outbox row ID and attempt number. No wall-clock value
is generated during serialization, which keeps equivalent publication input
deterministic for tests and retries.

### NATS message identity

The stable document key/idempotency key is the JetStream `Nats-Msg-Id` for
both direct publisher calls and outbox retries. It remains unchanged across
attempts, while a different onboarding OTP receives the existing OTP-specific
key.

### Compatibility

Legacy fields remain top-level and are populated from the canonical values:
`doc_type`, `id`, `branch_id`, `db_name`, and optional `otp`. The NATS subject
continues to use the legacy document type, tenant/database name, and branch
segments. No daemon code or nested-only envelope is introduced.

## Alternatives considered

1. **Make the daemon decode a new nested payload first** — rejected because the
   task explicitly excludes daemon changes and would break old deployments.
2. **Add a new outbox insert/update round trip to persist the generated row ID
   in the JSON** — rejected because it adds transaction complexity and is not
   needed when the committed drainer can enrich the outbound copy.
3. **Use a random event ID as `Nats-Msg-Id`** — rejected because retries would
   not deduplicate.
4. **Read `csr_computer_number` from branch configuration in every handler** —
   rejected for this lane because it widens handler SQL and changes business
   transaction behavior; the explicit EGS override leaves that integration
   available without making it mandatory here.

## Remaining uncertainty

The deployed daemon's implementation is outside this repository. Compatibility
is therefore validated against the known five-field JSON contract and focused
decode tests, not by modifying or running daemon code.
