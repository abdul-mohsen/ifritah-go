# Research: goose migration review fix

## Evidence inspected

### Current source

- `pkg/db/goose/00001_baseline.sql` already creates `branch_sequence` with
  `(branch_id, scope)` as the primary key and `last_value` defaulting to zero. The baseline
  also defines `bill.sequence_number` and `uq_bill_branch_seq`.
- `pkg/db/goose/00002_bill_identity.sql` through `00010_zatca_invoice_record.sql` are present
  and numbered contiguously from 1 through 10.
- `pkg/db/goose/migrations_test.go` currently enforces the filename shape, requires a Down
  section with SQL for every version after the baseline, and requires exact contiguity starting
  at `BaselineVersion` (1).
- `pkg/db/goose/00008_bill_number_counter.sql` backfills one `scope='bill'` row per branch
  with a non-null existing bill sequence, and raises an existing counter with
  `GREATEST`. Its Down currently executes `DELETE FROM branch_sequence WHERE scope = 'bill'`,
  which can delete rows that existed before this migration or were written by later sales.
- `pkg/db/goose/bill_number_test.go` currently verifies the backfill and allocation on a fresh
  database, but expects Down to remove every bill counter and does not seed pre-existing
  counters or unrelated scopes.
- `cmd/migrate/main.go` currently only exposes `up`, `down`, `roundtrip`, `status`, and
  `version`. Its `up` command does not distinguish an empty/fresh database from an existing
  database that was built outside goose.
- `pkg/db/goose/README.md` currently says there is no adoption path, although the command is
  used against tenant databases that may predate goose.

### Repository history

- Commit `5ef930f` introduced `00008_bill_number_counter.sql` and the bill-number parity test.
  The commit explicitly says the migration seeds counters from existing bills and that CI
  exercises the goose path.
- Commit `e5bb4e8` added `00010_zatca_invoice_record.sql`; its verification notes say the
  goose roundtrip reaches version 10. Thus removing or renumbering `00008` would leave a
  version gap and could invalidate an already-applied goose history.
- Commit `ad8f9a5` previously implemented an explicit `migrate adopt` command. Its `Adopt`
  function refused managed databases, empty databases, and schema drift, and recorded versions
  0 and 1 only after an exact baseline schema-digest comparison.
- Commit `562efa2` removed that adoption command and its digest files while choosing a
  one-path/new-database-only policy. No repository evidence proves that all existing tenant
  databases were discarded or that version 8 was never applied.
- The current branch is based on the history containing those migration commits and has no
  deployment evidence that makes renumbering or deleting an applied migration safe.

## Decisions

1. Keep `00008` and versions 9 and 10 unchanged in number. The current contiguous-version test
   remains valuable; changing the test to permit a gap would hide an unsafe migration history.
2. Restore an explicit, digest-checked `migrate adopt` path from the repository's prior
   implementation. `migrate up` remains the fresh/managed-database path; it does not guess that
   an arbitrary existing schema is the baseline.
3. Keep the existing `00008` Up backfill, including its `GREATEST` protection. Add regression
   coverage for lower and higher pre-existing counters and for null sequence values.
4. Make `00008` Down deliberately non-destructive (`SELECT 1`). A data backfill cannot
   distinguish rows it inserted from rows created before or after it without a provenance table,
   and deleting bill counters can cause number reuse. Leaving the baseline table's data intact
   is the safe rollback contract; re-applying `00008` remains idempotent.
5. Update the migration README and tests to document the fresh/adopt distinction and the
   intentionally non-reversible data backfill.

## EGS reservation ownership research

- The current worktree contains no `pkg/egschain` package, no `egs_chain.sql` query file, and no
  backend SQL reference to `icv_reservation_key`, `icv_reservation_token`, `reserved_icv`, or
  `reservation_lease_expires_at`; the generated model is stale and is the only non-migration
  reference.
- Deployment contract evidence identifies `queue.SQLChain` as the current runtime chain owner.
  Commit `0b21331`/the pre-removal source shows the complete SQL protocol that used the columns:
  row locking, booking, owner-token/key guarded commit, owner-token/key guarded release, and
  lease takeover. Commit `562efa2` removed that backend package while explicitly stating that
  the EGS chain belongs to the external daemon and that the reservation columns remain daemon
  storage.
- No daemon source tree is present in the requested worktree, so the current external daemon
  implementation cannot be inspected here. The repository history nevertheless provides a
  specific daemon-owned SQL contract, while no evidence proves that the columns are unused.

### EGS decision

Retain the 00005 columns and establish `queue.SQLChain` in the external ZATCA daemon as their
sole owner. Do not add
a removal migration based only on the backend package's absence: deleting a column used by the
daemon would be an irreversible cross-repository break, and the source history explicitly
preserves these columns for daemon storage. Remove the stale `pkg/egschain` ownership comments,
document the daemon contract, and add backend-side MySQL fixtures that exercise the SQL
invariants. The lack of the daemon repository remains an explicit release risk.

## Remaining uncertainty

- The digest-based adoption test requires `GOOSE_PARITY_DSN` and a supported MySQL server; it
  cannot prove an external tenant's history without that database. The no-DSN test run will
  therefore skip parity/adoption tests.
- Existing tenants with schema drift must be repaired or explicitly rejected; this change does
  not attempt an automatic repair.
