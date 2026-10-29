# T016: Atomic Per-EGS Invoice Sequence and Hash Ownership

## User Story

As the backend and ZATCA daemon, I need one tenant/branch/EGS-owned sequence
and prior-hash contract so concurrent document processing cannot fork the
fiscal chain or assign a number from another branch.

## Acceptance Scenarios

1. Posted invoices use a branch-local atomic counter.
2. Invoice, credit-note, and debit-note chain reservations share one ICV
   stream for the same branch/EGS.
3. A reservation is idempotent for the same stable document key.
4. A different owner cannot commit or release the reservation.
5. Existing T014 envelope and outbox identity fields remain compatible.
6. No daemon XML chain or debit domain code is changed.

## Scope

See `requirements.md` for the complete in-scope/out-of-scope boundary,
functional requirements, risks, and acceptance criteria.

## Related Artifacts

- `research.md`
- `design.md`
- `data-model.md`
- `contracts/zatca-chain.md`
- `quickstart.md`
- `tasks.md`
- `STATUS.md`
