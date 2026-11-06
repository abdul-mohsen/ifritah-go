# Research: Deterministic EGS Reservation Contract Test

## Repository and branch evidence

- The requested worktree is `feat/b6-credit-debit-2` at `bcc33e5` and was clean
  before this feature's artifacts were added.
- The target branch contains migration `00005_egs_chain_reservation.sql`, but it
  does not yet contain `pkg/db/goose/egs_chain_reservation_test.go`.
- The current #105 atomic-sale worktree contains that contract test in commit
  `38d4f00`; the test was introduced after the target branch forked. The fix is
  therefore scoped to adding the corrected contract test and its directly related
  migration ownership documentation in the requested worktree, rather than editing
  the concurrent worktree.

## Production protocol evidence

- The current daemon implementation is
  `zatca_integration.git/worktree/zatca-z9-notes/queue/chain.go`.
- `SQLChain` exposes an injectable `Now func() time.Time`; `Book` compares the
  stored lease against that clock.
- A different request key is rejected only while the existing lease is live:
  `row.ownerKey.Valid && !row.ownedBy(booking.RequestKey) && row.leaseLive(now)`.
  Once the lease is expired, the new request receives the same next ICV.
- A same-key live booking is resumed without changing the token/ICV, while a
  takeover after expiry gets the new request's token. Commit and release remain
  guarded by the booking key/token.
- This confirms that the contract test's different-document expired takeover is
  intentional for the current daemon. The helper's unconditional different-key
  busy check is the defect; changing production code is unnecessary.

## Historical evidence and compatibility boundary

- The removed backend `pkg/egschain` implementation in commits `0b21331` and
  `2fa4ba6` deliberately kept an expired reservation for another document busy.
- Commit `562efa2` removed that backend runtime owner and left the reservation
  columns for the external daemon. The current daemon's lease protocol is therefore
  the authoritative behavior for this contract.
- The old same-document-only rule must not be reintroduced into the current
  daemon contract merely because the target backend branch still has older migration
  wording.

## Failure mechanism

The contract-test helper currently does this:

1. If a reservation key exists and differs from the requested document, return
   `errEGSReservationBusy`.
2. Only for the same key, inspect whether the lease is live.

The expired takeover calls the helper with `document-2` after setting the stored
lease to an old timestamp. The helper returns busy before inspecting expiration,
so the assertion fails independently of wall-clock timing. The helper does not
match `queue.SQLChain.Book`.

The test also derives `now` from `time.Now()` and opens a fixed database name.
Those choices are unnecessary sources of timing and cross-process state coupling,
even though the immediate unconditional busy check is the deterministic failure.

## Resolved decisions

| Requirement | Evidence and decision |
|---|---|
| FR-1, FR-2 | Use one fixed test clock and an explicit expired lease value; no sleep or polling. |
| FR-2, FR-3 | Match the current daemon rule: only a live reservation blocks another document; owner key/token checks remain on release/commit. |
| FR-4 | Give the contract test a process-qualified database name while retaining `openFreshGooseDB`'s fresh create/drop lifecycle. |
| FR-5 | Keep assertions for same ICV takeover, stale-token refusal, owner release, owner commit, and post-commit ICV progression. |
| NFR-2 | Use UTC whole-second timestamps compatible with MySQL `DATETIME` and the existing `parseTime=true` DSN. |
| NFR-4 | Add only the contract test, the migration owner comment, the owner-name migration assertion, and scoped Spec Kit artifacts. |

## Validation constraints

- `GOOSE_PARITY_DSN` is unset in this environment, so the real MySQL contract test
  cannot execute locally unless a supported DSN is supplied by the test environment.
- The package's non-MySQL tests can still be run, and the contract test should
  compile and skip cleanly when the DSN is absent.
- No test is run in the concurrent #105 worktree; all edits and validations use the
  requested target worktree.
