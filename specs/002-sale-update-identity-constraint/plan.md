# Plan

## Phase 1: Artifacts and evidence

- Keep the scoped requirements, research, design, and acceptance checklist
  aligned with the existing B5 sale identity schema.
- Confirm the worktree remains clean before implementation.

## Phase 2: Surgical implementation

- Add a sale-update-specific constraint classifier for
  `uq_bill_pos_request`.
- Replace only the `writeSale` duplicate gate with that classifier.
- Do not alter the shared `IsDuplicate` helper or unrelated handlers.

## Phase 3: Regression coverage

- Make existing race injections identify the request-identity key.
- Add an unrelated unique-key failure test that proves the generic database
  error path is retained.

## Phase 4: Validation and convergence

- Run focused sale-handler tests.
- Run the backend test suite and formatting/diff checks.
- Review the source diff and artifacts for scope, then record convergence and
  any blocker in `STATUS.md`.
