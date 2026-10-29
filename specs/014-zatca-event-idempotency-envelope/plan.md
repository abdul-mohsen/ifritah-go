# T014 Implementation Plan

## Technical Context

- Language/runtime: Go 1.26.5 (`go 1.25` module/toolchain declaration).
- Messaging: NATS JetStream via `github.com/nats-io/nats.go`.
- Persistence: MySQL/sqlc durable outbox from T007.
- Existing contract: five top-level JSON fields consumed by a deployed daemon.
- Generated SQL output is ignored and must be regenerated before validation.

## Constitution Check

No project constitution file exists in the base commit. The plan follows the
repository's established constraints: additive changes, source SQL as the
authority, no hand-edited generated output, deterministic tests without live
services, and no frontend/daemon changes.

## Phase 0: Research

- Inspect T007 publisher/outbox paths and schema.
- Inspect historical versioned-contract commit `d52c8c4`.
- Resolve outbox-ID timing, attempt semantics, EGS identity, and compatibility.

## Phase 1: Design

- Extend the source-compatible publisher message with canonical envelope fields.
- Enrich the outbound copy from the committed outbox row.
- Use the stable document key as `Nats-Msg-Id`.
- Add focused capturing-publisher regressions and update README.

## Phase 2: Implementation

1. Add red publisher tests and record their failure at `d4069f4`.
2. Implement envelope construction, normalization, compatibility aliases, and
   deterministic identity.
3. Route outbox publication through row-aware enrichment.
4. Format and regenerate sqlc output.

## Phase 3: Validation and convergence

Run focused tests, build, vet, full tests, diff checks, artifact consistency
review, and convergence review. Commit one isolated T014 commit with the
required co-author trailer and do not push.

## Risks

- The daemon is not in this worktree; compatibility must be proven by payload
  shape and field values.
- EGS configuration is not part of the outbox row; the deterministic
  branch-scoped fallback avoids a new read/business-state dependency.
- The generated sqlc warning may remain pre-existing and must be reported
  exactly if vet reproduces it.
