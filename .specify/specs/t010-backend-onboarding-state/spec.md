# T010 Backend Onboarding State Specification

The backend must preserve T026's strict onboarding boundary while making
onboarding progress durable and observable. A valid administrator request
creates one guarded branch attempt in `queued` state, with a separate worker
step, monotonically increasing attempt number, retry/error fields, and
timestamps. The branch's existing ZATCA GET endpoint exposes only this
non-secret metadata. Later daemon updates may move the attempt through
`compliance` and `production` to `done`, or to `failed` with durable error
evidence. Duplicate active requests must not create another attempt.

The branch update and attempt insert commit together before the existing queue
publisher is called. Publisher failure is persisted as a failed attempt. No
daemon, frontend, outbox, credentials, or OTP persistence is part of T010.

See `requirements.md`, `research.md`, `design.md`, `plan.md`, and `tasks.md`
for traceability and validation evidence.
