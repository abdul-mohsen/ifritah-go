# Research

## Repository evidence

### Sale update error path

- `pkg/handlers/bill_sale.go:126-136` handles the error returned by
  `updateSaleBill`.
- The current branch checks `IsDuplicate(err)`, which only checks MySQL error
  number `1062`. Any duplicate key therefore enters `answerLostRace`, rolls
  back, and performs a request-identity lookup.
- A non-duplicate error follows the existing
  `http.StatusInternalServerError` path with the original error.

### Authoritative request-identity constraint

- `pkg/db/goose/00002_bill_identity.sql:12` declares the request-identity
  unique key as `uq_bill_pos_request` on `(pos_id, bill_request_id)`.
- `pkg/db/goose/bill_write_test.go:68-73` verifies that a duplicate sale
  identity reports MySQL error `1062` and includes `uq_bill_pos_request` in the
  driver message.
- This migration and parity test are the repository's authoritative source for
  the constraint identifier; no schema change is required.

### Existing tests and injection seam

- `pkg/handlers/sale_test.go:340-378` already covers the two true race outcomes:
  replaying the winner and returning the duplicate message when no winner is
  visible.
- Those tests currently inject only `Number: 1062, Message: "Duplicate entry"`,
  which does not identify the key and would incorrectly represent an unrelated
  duplicate after the classifier is narrowed.
- The handler tests use `sqlmock`, so both constraint-specific error messages
  can be injected without a live database.

## Resolved unknowns

- **Exact handler:** `writeSale` in `pkg/handlers/bill_sale.go`.
- **Exact constraint:** `uq_bill_pos_request`.
- **Error representation:** wrapped or direct `*mysql.MySQLError`, identified
  with `errors.As`; the driver message contains the constraint name.
- **Test seam:** existing `sqlmock` expectations in `pkg/handlers/sale_test.go`.
- **Expected unrelated behavior:** the existing generic
  `http.StatusInternalServerError` branch should remain in use for a duplicate
  on another key.

## Design alternatives considered

1. **Keep checking only MySQL error number 1062.** Rejected because it is the
   reported bug and cannot distinguish request identity from other unique
   constraints.
2. **Parse duplicate values or request fields from the driver message.**
   Rejected because it is more brittle and unnecessary; the schema gives the
   stable constraint identifier.
3. **Add a new database query or migration.** Rejected because the constraint
   already exists and the fix is only error classification.
4. **Match the existing request-identity constraint name in addition to error
   number 1062.** Selected as the smallest compatible change. It preserves
   wrapped MySQL error handling and changes only the sale update race decision.

## Remaining uncertainty

None for the requested behavior. The error message is supplied by the MySQL
driver and the repository already asserts the same key name in its schema
parity test.
