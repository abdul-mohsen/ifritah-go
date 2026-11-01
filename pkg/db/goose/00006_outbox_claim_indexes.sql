-- +goose Up
-- The claim reads due events in the order of an index, so it stops at its batch size and locks
-- only the rows it returns. idx_outbox_claim cannot give that order. The old claim sorted every
-- due row and, under REPEATABLE READ, locked every published row it walked past. Pending events
-- are now read through idx_outbox_due, and expired leases through idx_outbox_lease. Inserts
-- maintain one more secondary index than before.
ALTER TABLE outbox
  ADD KEY idx_outbox_due (status, next_attempt_at, id),
  ADD KEY idx_outbox_lease (status, lease_expires_at, id),
  DROP KEY idx_outbox_claim;

-- +goose Down
ALTER TABLE outbox
  ADD KEY idx_outbox_claim (status, next_attempt_at, lease_expires_at, id),
  DROP KEY idx_outbox_lease,
  DROP KEY idx_outbox_due;
