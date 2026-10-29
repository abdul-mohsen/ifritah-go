# T014 Status

## Completed

- T014A-REQ: requirements recorded before source changes.
- T014A-RESEARCH: T007 baseline, historical versioned-contract conventions,
  EGS schema evidence, outbox-ID timing, attempt semantics, and migration
  compatibility researched.
- T014A-DESIGN: envelope, enrichment, identity, compatibility, and test design
  recorded.
- T014A-PLAN: plan, data model, contract, quickstart, and dependency-ordered
  tasks recorded.
- T014B-RED: focused publisher regressions fail at `d4069f4`; the legacy
  payload omits `contract` and the other required envelope fields.
- T014C-IMPLEMENT: added the additive versioned message envelope, canonical
  identity fields, deterministic stable-key NATS identity, and legacy aliases.
- T014D-OUTBOX: committed outbox publication now enriches payloads with row ID,
  event ID, stable key, and 1-based attempt metadata without changing business
  state or persistence schema.
- T014E-VALIDATE: sqlc generation, formatting, build, focused tests, and the
  full Go test suite pass; vet has one pre-existing ignored sqlc warning.
- T014F-CONVERGE: requirements, design, plan, tasks, source scope, migration
  compatibility, and tests reviewed; no daemon/reconciliation or business
  state changes found.
- T014G-COMMIT: isolated local commit `a1b43d8` created on
  `fix/zatca/h2-event-idempotency-envelope`; no push or PR created.

## In Progress

- None.

## Blocked

- None currently.

## Backlog

- None.

## Risks

- The deployed daemon is outside this repository; compatibility is proven by
  retained top-level fields and decode tests.
- The current outbox row does not persist EGS configuration, so the envelope
  uses an explicit value when supplied and a deterministic tenant/branch
  fallback.
- Ignored generated sqlc output must be regenerated before every validation
  run.

## Next Actions

1. No further T014 implementation action remains; do not push or create a PR.

## Validation Evidence

- Red focused command:
  `rtk gofmt -w pkg/handlers/zatca_envelope_test.go; rtk go test ./pkg/handlers -run 'TestZatcaPublisherEmitsVersionedEnvelopeAndDeterministicIdentity|TestZatcaOutboxPayloadContainsEnvelopeContextAndLegacyAliases' -count=1`
- Red result at `d4069f4`: `0 passed, 2 failed`; both tests reported the
  legacy payload omitted `contract` (and therefore the required envelope).
- Focused green command:
  `rtk go test ./pkg/handlers -run 'TestZatcaPublisher|TestZatcaOutboxEnvelope|TestZatcaOutboxPayload|TestZatcaOnboardingEnvelope' -count=1`
- Focused green result: `4 passed`.
- Handler regression command:
  `rtk go test ./pkg/handlers -count=1`
- Handler result: `105 passed`.
- Generation: `rtk sqlc generate` — success.
- Formatting: `rtk gofmt -l pkg/handlers/pub.go pkg/handlers/zatca_envelope.go pkg/handlers/zatca_outbox.go pkg/handlers/zatca_envelope_test.go` — no output.
- Build: `rtk go build ./...` — two attempts hit a Windows temporary
  `a.out.exe` unlink lock; `rtk go build ./pkg/...` and
  `rtk go build -o t014-build.exe .` both succeeded, and the local artifact
  was removed.
- Full tests: `rtk go test ./... -count=1` — `121 passed in 6 packages`.
- Vet: `rtk go vet ./...` — blocked only by the pre-existing ignored generated
  sqlc warning `pkg/db/gen/models.go:994:2: struct field _0 has json tag but
  is not exported`; no T014 generated output was hand-edited.
