# ifritah-go

## Sale-line discounts

Sale requests send each line's discount through `AddProductToBill` to the existing `bill_product.discount` column. The backend remains the source of truth for line and bill totals; do not calculate or persist accounting totals in the frontend.

## Backend PR order

The credit-source endpoint branch (#108) is stacked on the atomic-sale branch (#105). Integrate #105 before #108 so the stacked branch keeps the intended base.