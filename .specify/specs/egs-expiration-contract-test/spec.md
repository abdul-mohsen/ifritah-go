# Feature Specification: Deterministic EGS Reservation Contract Test

## User scenarios and testing

### User Story 1 — Prove live reservation ownership (Priority: P1)

The contract test proves that a live reservation is returned to its same document
without changing its token or ICV, while another document is refused.

**Acceptance scenarios**

1. **Given** a live reservation for document 1, **when** document 1 retries,
   **then** the original token and ICV remain unchanged.
2. **Given** a live reservation for document 1, **when** document 2 tries to book,
   **then** the booking is refused as busy.

### User Story 2 — Prove deterministic expired-lease takeover (Priority: P1)

The contract test proves that a reservation whose lease is explicitly before the
test clock can be taken over by the next document without waiting in real time.

**Acceptance scenarios**

1. **Given** document 1's lease is explicitly expired, **when** document 2 books,
   **then** document 2 receives the same reserved ICV with a new token.
2. **Given** document 2 has taken over, **when** document 1's stale token tries to
   release or commit, **then** it cannot change the reservation.

### User Story 3 — Keep test state isolated and the chain safe (Priority: P1)

The contract test can run repeatedly or alongside another worktree's test process
without database-name collisions, and owner-guarded release/commit still preserve
the committed chain.

**Acceptance scenarios**

1. **Given** two test processes use the same MySQL server, **when** they create their
   contract databases, **then** one process does not drop the other's database.
2. **Given** an owner releases or commits a reservation, **then** the committed ICV
   and previous hash follow the reservation contract and the next reservation gets
   the expected ICV.

## Requirements

- **FR-1:** The test uses controlled, explicit lease state for live and expired cases.
- **FR-2:** The test reflects the current daemon's rule that only a live reservation
  blocks a different document.
- **FR-3:** The test preserves owner/token guards for release and commit.
- **FR-4:** The test database is isolated from concurrent test processes.
- **FR-5:** The migration documentation identifies the external daemon as the runtime
  reservation owner.

## Success criteria

- **SC-1:** The focused MySQL contract test passes without sleeping when a DSN is
  available.
- **SC-2:** The test compiles and skips cleanly with a clear message when no DSN is
  available.
- **SC-3:** No production lease or schema behavior changes.
- **SC-4:** Repeated focused/package Go tests and static checks pass.

## Constraints

- Work only in the requested backend worktree.
- Do not push or perform GitHub writes.
- Do not use arbitrary sleeps, generated `pkg/db/gen` edits, or unrelated changes.
