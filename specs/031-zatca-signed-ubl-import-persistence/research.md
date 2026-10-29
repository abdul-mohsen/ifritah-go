# T031 research

## Evidence inspected

1. Backend commit `98ae215` is the requested base and contains the durable
   `zatca_submission`, `zatca_event_outbox`, branch/EGS sequence, and debit
   lanes. Existing monitor and publisher code is tenant-scoped by
   `merchant_id`.
2. `pkg/db/schema/schema.sql`, migrations `0008`–`0012`, and sqlc queries show
   outbound submission state already has provider/outbox semantics. Reusing
   `zatca_submission` for supplier intake would conflate review state with
   provider outcome and violate the requested separation.
3. Existing routes in `main.go` are mounted under a JWT `authorized` group.
   `GetSessionInfo(c).id` is the established authenticated merchant boundary.
4. T030 commit `b0099f7` in the separate `zatca_integration` repository exposes:
   `ImportRequest`, `ImportResult`, `XMLSourceEvidence`, `UBLImport`,
   `ValidationState`, `ValidationIssue`, `SignatureEvidence`, and stable
   failure classes (`malformed`, `unsafe`, `oversized`, `required_fields`,
   `signature`, `trust_unavailable`, `schema`, `unsupported`). The result
   retains `OriginalBytes`, `Source`, `Canonical`/`Parsed`, and failure
   evidence but intentionally performs no persistence.
5. Historical backend intake commits `991aeff`/`ba7629c` confirm the product
   pattern of immutable supplier evidence, append-only transitions, review
   gating, and authenticated `incoming_purchase_document` routes. Those
   commits are on a different branch and use a broader purchase-intake domain;
   this task adopts only the relevant evidence/lifecycle ideas and does not
   import unrelated OCR, posting, or frontend work.

## Resolved unknowns and decisions

| Unknown | Evidence/decision | Requirement |
|---|---|---|
| Where should import state live? | Separate `zatca_import_*` tables; existing submission tables represent outbound provider state. | FR-3, FR-11 |
| How does backend align with T030 without a cross-repo dependency? | Define JSON-compatible request/response models locally and persist the T030 fields read-only. | FR-2, FR-3 |
| How are exact bytes retained? | Store `MEDIUMBLOB` bytes when supplied and always store a verified hash/reference; no object-store dependency is added. | FR-2 |
| How are validation issues queried? | Append one row per issue plus full validation JSON in immutable evidence. | FR-4 |
| What is the initial workflow state? | `review`, because T030 explicitly marks trust/unsupported/validation outcomes for review and this task must not auto-post. | FR-5, FR-6 |
| What does post mean? | Import workflow completion with an optional caller reference only; no purchase bill/accounting/outbound submission mutation. | FR-7, FR-11 |
| What auth boundary applies? | Existing JWT session `merchant_id`; every query includes it. | FR-1, FR-9 |

## Alternatives considered

- **Reuse `zatca_submission`:** rejected because provider outcome states and
  outbound artifacts are not import review state.
- **Persist only a file/object reference:** rejected for this bounded backend
  lane because T030 returns exact bytes and the acceptance criteria require
  evidence to survive without a storage adapter.
- **Call or vendor the T030 parser:** rejected because T030 is a separate
  repository and this task explicitly requests read-only API alignment.
- **Create a daemon/NATS event for import:** rejected; import review/post/reject
  is a synchronous authenticated backend workflow and daemon changes are out
  of scope.
- **Mutate purchase bills on post:** rejected; posting semantics are deliberately
  limited to durable import workflow state in T031.

## Remaining uncertainty

Live MySQL migration execution, MEDIUMBLOB/JSON limits, concurrent transition
locking, and any production object-store retention policy require a deployment
rehearsal. sqlmock and local tests cannot prove those properties.
