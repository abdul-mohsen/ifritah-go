# Specification: goose migration review fix

This feature restores a safe migration contract for the atomic-sale backend. It preserves
goose versions 00001 through 00010, backfills existing bill counters without lowering known
counters, distinguishes fresh/managed migration from explicit existing-database adoption, and
never deletes branch sequence data during the 00008 rollback. It also establishes the external
ZATCA daemon as the sole owner of the 00005 EGS reservation columns after the backend
`pkg/egschain` package was removed, with SQL-contract coverage for booking, release, commit, and
re-delivery behavior.

The detailed requirements and acceptance criteria are in
[`requirements.md`](requirements.md); evidence and decisions are in
[`research.md`](research.md).
