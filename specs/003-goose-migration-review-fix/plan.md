# Implementation plan: goose migration review fix

## Technical context

- Go 1.25/1.26 toolchain, MySQL, and `pressly/goose/v3`.
- Tenant migrations are embedded from `pkg/db/goose`.
- Existing parity tests use `GOOSE_PARITY_DSN`; package tests otherwise run without a database.
- Version renumbering is unsafe because history proves 00008 was introduced and later migrations
  depend on the contiguous goose path.
- The external daemon is the owner of the 00005 reservation columns; no current daemon source is
  present in this worktree, so backend tests verify the established SQL contract rather than
  introducing a second runtime owner.

## Plan

1. Restore the previously reviewed digest-backed adoption implementation and command.
2. Keep migration versions unchanged and change 00008 Down to a safe no-op.
3. Update 00005 comments and migration documentation to name the external daemon owner.
4. Add static and MySQL regression tests for version/Down, bill-counter, and EGS SQL behavior.
5. Document fresh/managed versus explicit adoption behavior.
6. Run focused tests, then package/repository validation, self-review, convergence, and commit.

## Constitution check

No repository constitution file is present in the existing `.specify` metadata. This plan
therefore follows the explicit task constraints: preserve migration history, fail closed on
ambiguous adoption, keep changes surgical, and validate before committing.

## Design references

- [`requirements.md`](requirements.md)
- [`research.md`](research.md)
- [`design.md`](design.md)
- [`tasks.md`](tasks.md)
- [`quickstart.md`](quickstart.md)
