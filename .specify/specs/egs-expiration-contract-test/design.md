# Design: Deterministic EGS Reservation Contract Test

## Technical context

- The backend owns the Goose migration that stores EGS reservation columns.
- The external `queue.SQLChain` daemon owns booking, lease expiry/takeover, release,
  and commit behavior.
- The backend contract test uses a real MySQL database because row locks, `DATETIME`
  lease comparisons, and owner-guarded writes are the behavior under test.

## Test data flow

1. Skip when `GOOSE_PARITY_DSN` is unset.
2. Open a process-qualified fresh Goose database and migrate through version 5.
3. Insert one company, branch, and EGS row, then seed `last_icv=7` and a previous
   hash.
4. Use a fixed UTC whole-second test clock and a live lease derived from it.
5. Verify the reservation CHECK constraint and `NOWAIT` row-lock behavior.
6. Book document 1, verify ICV 8 and the lease, and verify live same-document
   redelivery preserves the original token.
7. Verify a different document is busy while the lease is live.
8. Set the lease explicitly to a timestamp before the fixed clock, then book the
   different document and verify it takes the same ICV with its new token.
9. Verify a stale token cannot release the takeover, the owner can release it, and
   the committed chain remains unchanged.
10. Book and commit a later document, verify the chain advances to ICV 8, and verify
    a subsequent reservation receives ICV 9.

## Helper contract

`bookEGSReservation` mirrors `queue.SQLChain.Book`:

- A different key is busy only when the stored lease is still after the supplied
  `now`.
- A same-key live booking returns the existing reservation without changing it.
- An expired reservation, regardless of key, is overwritten with the new key/token,
  the same reserved ICV, and the supplied lease.
- The helper does not change production code or bypass owner guards.

## State isolation

- The test database name is derived from a stable prefix and the current test
  process ID. This prevents two worktrees/processes using the same MySQL server from
  dropping each other's database.
- The test still drops the database during cleanup through `openFreshGooseDB`.
- The branch ID and document values remain fixed because the database itself is
  isolated; no random data or wall-clock state is needed.

## Migration documentation

Update the directly related `00005_egs_chain_reservation.sql` comment and
`pkg/db/goose/README.md` to identify `queue.SQLChain` as the runtime owner, and add
a migration test that rejects stale references to the removed backend `pkg/egschain`
owner. This is documentation and contract protection only; the schema and runtime
production code are unchanged.

## Failure handling

- A missing DSN skips the MySQL-only contract test with an explicit reason.
- Any migration, insert, lock, lease, or owner-guard failure fails the test with the
  relevant operation.
- No fallback sleep or retry is added.

## Test strategy

- Focused: `go test ./pkg/db/goose -run 'TestEGSReservationContract|TestEGSReservationMigrationNamesDaemonOwner'`.
- Package: `go test ./pkg/db/goose`.
- Relevant repository checks: `go test ./...`, `go vet ./...`, `gofmt -l` on changed
  Go files, and `git diff --check`.
- With a MySQL DSN, repeat the focused contract test to verify process/state
  isolation; without one, record the skip limitation.

## Requirement traceability

| Requirement | Design response |
|---|---|
| FR-1 | Fixed clock and explicit pre-clock lease value. |
| FR-2 | Helper checks lease liveness before rejecting a different key and asserts both live busy and expired takeover. |
| FR-3 | No production code change; owner/token release and commit assertions remain. |
| FR-4 | Process-qualified fresh database. |
| FR-5 | Existing contract assertions cover ICV preservation, stale owner refusal, release, commit, and progression. |
| NFR-1 | No sleeps or polling. |
| NFR-2 | UTC whole-second `time.Time` values and MySQL `DATETIME` precision. |
| NFR-3 | Focused/package/full Go validation. |
| NFR-4 | Surgical test, migration comment/assertion, and scoped artifacts only. |
