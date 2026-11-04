# Bill Credit-Source API Contract

## Request

- **Method/path:** `GET /api/v2/bill/{id}/credit-source`
- **Authentication:** Existing JWT middleware for the v2 API.
- **Authorization:** Caller must have access to the bill's store.
- **Path parameter:** `id` is a positive bill ID.
- **Request body:** None.

## Success response

`200 OK`

```json
{
  "bill_id": 42,
  "branch_id": 7,
  "amount": "28.75",
  "lines": [
    {
      "bill_product_id": 5,
      "quantity": "1.000",
      "price": "10.00",
      "vat": "15.00"
    }
  ]
}
```

- IDs are JSON integers.
- `amount`, `price`, and `vat` are decimal strings with two fractional digits.
- `quantity` is a decimal string with three fractional digits.
- `amount` and `lines` describe the full remaining credit only; already credited quantities and amounts are excluded.
- No success response is returned for missing/invalid source metadata or unreconciled history.

## Error behavior

- `400 Bad Request`: invalid route ID or caller lacks store access (the latter preserves existing handler behavior).
- `404 Not Found`: bill does not exist or is deleted.
- `409 Conflict`: draft, fully credited, missing required source metadata, or remaining credit cannot be proven.
- `500 Internal Server Error`: unexpected database or transaction failure.

Error responses do not include a credit-source payload.

## Preview semantics

The GET does not reserve or modify credit. Its result may become stale after the read transaction ends. `POST /api/v2/bill/credit` remains authoritative and revalidates availability transactionally.
