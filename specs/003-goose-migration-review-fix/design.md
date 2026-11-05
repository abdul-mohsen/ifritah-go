# Design: goose migration review fix

## Boundaries

- `pkg/db/goose` remains the sole tenant migration source.
- The numbered SQL files retain versions 1 through 10.
- `cmd/migrate` is the explicit operator boundary for fresh migration and legacy-schema
  adoption.
- Migration 00005's reservation columns remain in the tenant schema, owned exclusively by
  `queue.SQLChain` in the external ZATCA daemon; the backend package does not query or mutate them.

## Migration behavior

### Fresh and managed databases

`migrate up` remains the normal path for an empty database or a database already managed by
goose. It applies the baseline and pending numbered migrations in order. The existing
contiguous-version check remains unchanged so a missing numbered file is a test failure.

### Existing databases

`migrate adopt` is an explicit opt-in path for a database built from the pre-goose baseline.
Before writing any goose state it:

1. refuses a database that already contains goose version rows;
2. refuses an empty database;
3. computes information-schema digests for base tables, columns, indexes, foreign keys, and
   check-constraint names;
4. compares them with the checked-in digest of `00001_baseline.sql`; and
5. records goose versions 0 and 1 only after an exact match.

The operator then runs `migrate up` to apply migrations 2 through 10. Any drift is reported
without creating the goose version table, so repair is explicit rather than guessed.

### Bill counter backfill and rollback

`00008` keeps its existing idempotent Up statement. It inserts the highest non-null bill number
per branch and raises an existing `scope='bill'` counter without lowering it. It deliberately
does not create counters for branches with no numbered bills.

Because the backfill changes durable data and has no provenance for rows that may predate the
migration, its Down section is a documented no-op (`SELECT 1`). This prevents reuse of an
existing bill number and preserves counters created by later sales. Reapplying Up is safe due to
the `GREATEST` update.

### EGS reservation ownership

Migration 00005 is retained for compatibility with the daemon's branch-level chain protocol.
Comments identify `queue.SQLChain`—not the removed backend package—as the owner. The backend
parity test uses the same SQL shape to verify:

- a row lock serializes concurrent bookings;
- a live reservation is idempotent for the same document key and rejects another key;
- an expired reservation can be taken over with a new token without changing the ICV;
- release clears only the matching owner;
- commit advances `last_icv`, stores the signed hash, and clears the reservation only for the
  matching owner; and
- stale/re-delivered commit or release attempts affect zero rows.

The test is a schema/SQL-contract fixture, not a second runtime owner. The external daemon
implementation must remain coordinated with this contract.

## Test strategy

- Keep the static migration filename/version and Down-section contract test.
- Add a static regression that rejects destructive `branch_sequence` operations in `00008` Down.
- Extend the MySQL parity test to cover lower/higher pre-existing counters, null sequences,
  allocation results, and preservation of bill/other-scope rows after Down.
- Restore digest-backed adoption tests for baseline adoption, drift refusal, managed-database
  refusal, and empty-database refusal.
- Add an EGS reservation SQL-contract parity test and remove stale `pkg/egschain` references from
  migration comments.
- Run no-DSN package tests (parity tests skip) and, when available, the full MySQL parity suite.

## Compatibility, security, and performance

- No applied migration filename or version changes.
- Adoption reads only `information_schema` and does not mutate data until all checks pass.
- Digest comparison intentionally ignores MySQL-version-sensitive defaults, collations, and
  column order while checking the structural contract needed to identify the baseline.
- Counter backfill is one grouped query and one upsert; no new runtime allocation path is added.

## Rollout and rollback

1. Deploy the migration binary with the existing numbered files.
2. Run `migrate up` on fresh or goose-managed databases.
3. Run `migrate adopt` followed by `migrate up` for verified pre-goose databases.
4. Do not run `down` as a data rollback for `00008`; it preserves counter data by design.

If an adoption check fails, repair or replace the database outside this change and rerun the
explicit command. No automatic schema repair is attempted.

## Requirement traceability

| Requirement | Design element |
| --- | --- |
| Version stability | Preserve 00001–00010 and keep contiguous-version test |
| Counter backfill | Existing Up plus lower/higher/null regression fixture |
| Fresh path | `migrate up` documented and tested by existing parity suite |
| Existing/adoption path | Digest-backed `migrate adopt` and refusal tests |
| Non-destructive Down | `00008` Down no-op and static/MySQL regression tests |
| EGS ownership | 00005 daemon-owner comment and SQL-contract parity fixture |
| Documentation/validation | Migration README, quickstart, focused and repository checks |
