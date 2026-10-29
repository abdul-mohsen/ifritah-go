# T016 Data Model

## Branch invoice counter

`branch_sequence` (existing migration):

| Field | Meaning |
|---|---|
| `branch_id` | Tenant-local branch FK |
| `scope` | Fixed `bill` invoice-number scope |
| `last_value` | Last allocated branch-local invoice number |

Primary key: `(branch_id, scope)`.

## EGS chain state

`zatca_egs_chain` (T016):

| Field | Meaning |
|---|---|
| `branch_id` | Tenant-local branch FK |
| `egs_id` | Canonical EGS identity |
| `last_icv` | Last committed shared invoice counter value |
| `previous_invoice_hash` | Last committed signed-document hash |
| `reservation_key` | Stable T014 document key currently owning the row |
| `reservation_token` | Opaque owner token |
| `reserved_icv` | ICV assigned to the active owner |
| `reserved_previous_hash` | PIH returned to the active owner |
| `reserved_at` | Reservation creation time |
| `lease_expires_at` | Recovery eligibility time |
| `updated_at` | State update timestamp |

Primary key: `(branch_id, egs_id)`.

The active reservation fields are nullable together. A committed chain has no
owner. `last_icv` and `previous_invoice_hash` advance only on a matching
commit.

## State transitions

```text
unreserved(last_icv, previous_hash)
  -> reserved(reservation_key, token, reserved_icv, reserved_previous_hash)
  -> committed(last_icv=reserved_icv, previous_hash=new_hash, no owner)
  -> unreserved

reserved -> unreserved
  on matching release or expired-owner recovery
```

## Validation rules

- `branch_id > 0`.
- `egs_id` and `reservation_key` are non-empty and bounded.
- `reserved_icv = last_icv + 1` for a new reservation.
- `last_icv` is never decremented.
- Only the exact owner token and reservation key can commit/release.
- A stable key can recover its own active reservation without incrementing.
