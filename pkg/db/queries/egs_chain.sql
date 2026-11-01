-- EGS chain reservation (B4). Each statement runs inside the caller's transaction, through
-- pkg/egschain. A reservation is owned by its token. Every write after the lock also names the
-- owner, so a stale owner cannot change the chain. The branch row is read with NOWAIT: a row that
-- another transaction holds fails at once, and pkg/egschain returns ErrBusy instead of queueing.

-- name: LockEGSChain :one
SELECT branch_id,
       last_icv,
       previous_invoice_hash,
       icv_reservation_key,
       icv_reservation_token,
       reserved_icv,
       reservation_lease_expires_at
FROM branch_zatca_config
WHERE branch_id = sqlc.arg('branch_id')
FOR UPDATE NOWAIT;

-- name: StartEGSChainReservation :execresult
UPDATE branch_zatca_config
SET icv_reservation_key = sqlc.arg('reservation_key'),
    icv_reservation_token = sqlc.arg('reservation_token'),
    reserved_icv = sqlc.arg('reserved_icv'),
    reservation_lease_expires_at = sqlc.arg('lease_expires_at')
WHERE branch_id = sqlc.arg('branch_id');

-- name: CommitEGSChainReservation :execresult
-- last_icv is set before reserved_icv is cleared. MySQL applies SET assignments left to right.
UPDATE branch_zatca_config
SET last_icv = reserved_icv,
    previous_invoice_hash = sqlc.arg('previous_invoice_hash'),
    icv_reservation_key = NULL,
    icv_reservation_token = NULL,
    reserved_icv = NULL,
    reservation_lease_expires_at = NULL
WHERE branch_id = sqlc.arg('branch_id')
  AND icv_reservation_token = sqlc.arg('reservation_token')
  AND icv_reservation_key = sqlc.arg('reservation_key')
  AND reserved_icv = sqlc.arg('reserved_icv');

-- name: ReleaseEGSChainReservation :execresult
UPDATE branch_zatca_config
SET icv_reservation_key = NULL,
    icv_reservation_token = NULL,
    reserved_icv = NULL,
    reservation_lease_expires_at = NULL
WHERE branch_id = sqlc.arg('branch_id')
  AND icv_reservation_token = sqlc.arg('reservation_token')
  AND icv_reservation_key = sqlc.arg('reservation_key');
