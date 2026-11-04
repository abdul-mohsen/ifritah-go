# ifritah-go

## Bill credit-source preview

Authenticated callers with access to the bill's store can request `GET /api/v2/bill/{id}/credit-source` to retrieve the bill's full remaining credit:

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

Decimal fields are strings (`amount`, `price`, and `vat` use two fractional digits; `quantity` uses three). The response includes only credit remaining after prior notes. It is a read-only preview, not a reservation: `POST /api/v2/bill/credit` remains authoritative and revalidates availability transactionally.