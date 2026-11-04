# Data Model: Bill Credit-Source Preview

## Bill

| Field | Source | Validation/use |
|---|---|---|
| `id` | Bill row | Positive ID from the route; returned as an integer. |
| `state` | Bill row | Negative means deleted/not found; zero means draft; positive is eligible under existing note rules. |
| `branch_id` | Bill row | Required and nonzero; returned as an integer. |
| `store_id` | Bill row | Used by the existing caller/store authorization check. |
| `total` | Bill row | Basis for the remaining amount after prior credit-note amounts. |

## Bill product line

| Field | Source | Validation/use |
|---|---|---|
| `id` | `bill_product` row | Required, positive, unique within the source result; returned as `bill_product_id`. |
| `quantity` | `bill_product` row | Must be positive; prior credit quantities are subtracted. |
| `price` | `bill_product` row | Persisted price; must satisfy the existing credit-line amount rules for any remaining quantity. |
| `discount` | `bill_product` row | Used by `creditLines` to allocate the source discount proportionally and calculate remaining line totals. |
| `vat` | `bill_product` row | Required for every source line; returned with two decimal places. |

## Credit history

- Credit-note amounts are subtracted from the bill total using `creditRemaining`.
- Credit-note line quantities are aggregated by `bill_product_id`.
- A prior line must refer to an existing source line, have a positive quantity, and not exceed the billed quantity.
- Invalid/nonpositive amounts or line history that cannot be reconciled cause refusal.

## Preview response

| Field | Type | Meaning |
|---|---|---|
| `bill_id` | JSON integer | Source bill ID. |
| `branch_id` | JSON integer | Source bill branch ID. |
| `amount` | Decimal string, 2 places | Entire remaining creditable amount. |
| `lines` | Array | All source lines with positive remaining quantity. |
| `lines[].bill_product_id` | JSON integer | Source bill product ID. |
| `lines[].quantity` | Decimal string, 3 places | Source line quantity less all recorded credits. |
| `lines[].price` | Decimal string, 2 places | Original persisted unit price. |
| `lines[].vat` | Decimal string, 2 places | Original persisted VAT rate. |

The response exists only when `checkNoteTotal(amount, candidateLines)` proves that the complete remaining amount equals the remaining line totals.
