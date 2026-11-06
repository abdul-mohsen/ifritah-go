-- +goose Up
-- Fresh tenants have no bill history to backfill. The sale transaction creates a branch's
-- bill counter on its first sale, at 1, and increments it atomically thereafter. Keep this
-- version as a no-op marker so applied migration numbers remain stable.
SELECT 1;

-- +goose Down
-- Counters are created and advanced by sales, not by this migration. Leaving them intact on
-- rollback prevents a later sale from reusing a bill number.
SELECT 1;
