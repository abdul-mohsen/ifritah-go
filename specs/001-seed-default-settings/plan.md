# Implementation Plan: Seed Fresh Tenant Defaults

## Technical Context

- **Language/runtime:** Go 1.x backend with MySQL tenant databases.
- **Migration system:** Pressly Goose, embedded files under
  `pkg/db/goose`.
- **Schema ownership:** `pkg/db/goose/00001_baseline.sql` owns the tenant
  schema; `settings` is a key/value table with a unique key.
- **Current migration version:** `00010`.
- **Generated code:** SQLC output is not affected because this change adds
  data only.
- **Testing:** Goose integration tests use `GOOSE_PARITY_DSN`.

## Constitution Check

- Preserve the single Goose migration path: **pass**.
- Do not hand-edit generated `pkg/db/gen`: **pass**.
- Preserve existing tenant data: **pass**, using insert-if-absent and a
  non-destructive Down.
- Validate with focused tests plus build, vet, and gofmt: **pass**.

## Phase 0 — Research

Completed in `research.md`. Historical migrations, current handlers, schema
ownership, test helpers, and migration rules were inspected.

## Phase 1 — Design

Completed in `design.md`. The change is one data migration, one integration
regression test, a Goose README clarification, and feature artifacts.

## Implementation Sequence

1. Add `pkg/db/goose/00011_seed_default_settings.sql`.
2. Add `pkg/db/goose/settings_test.go`.
3. Update `pkg/db/goose/README.md` to document seeded defaults and rollback
   preservation.
4. Run focused tests, then package tests/build/vet/gofmt.
5. Review the diff, run convergence/self-review, and commit locally.

## Validation Commands

From `C:\ssda\chatGPT\ifritah\backend.git\worktree\backend-b13-check-2`:

```text
rtk go test -count=1 ./pkg/db/goose/...
rtk go test -count=1 ./pkg/handlers/...
rtk go build ./...
rtk go vet ./...
rtk gofmt -l .
```

With MySQL available, set `GOOSE_PARITY_DSN` and run the Goose package tests
without skipping the fresh-database regression.
