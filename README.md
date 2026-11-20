# ifritah-go

## Sale-line discounts

Sale requests send each line's discount through `AddProductToBill` to the existing `bill_product.discount` column. The backend remains the source of truth for line and bill totals; do not calculate or persist accounting totals in the frontend.

## ZATCA NATS connection

Local Compose uses its own `nats` service at `nats://nats:4222`. In deployment, set `NATS_URL` to the private NATS listener owned by the ZATCA daemon and set `NATS_USER`/`NATS_PASSWORD` to the same credentials configured on that daemon. The publisher applies credentials separately, so do not embed them in `NATS_URL`. The deployment `config.env` is the source for these Dokku variables.

## Backend PR order

The credit-source endpoint branch (#108) is stacked on the atomic-sale branch (#105). Integrate #105 before #108 so the stacked branch keeps the intended base.