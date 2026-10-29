-- T016 daemon-consumable, tenant-local EGS chain ownership contract.
-- Call the ensure/select/claim statements in one transaction. Commit and
-- release must retain the owner predicates shown below.

-- name: EnsureZatcaEGSChain :exec
INSERT INTO zatca_egs_chain (branch_id, egs_id)
VALUES (?, ?)
ON DUPLICATE KEY UPDATE branch_id = VALUES(branch_id);

-- name: GetZatcaEGSChainForUpdate :one
SELECT
  branch_id,
  egs_id,
  last_icv,
  previous_invoice_hash,
  reservation_key,
  reservation_token,
  reserved_icv,
  reserved_previous_hash,
  reserved_at,
  lease_expires_at,
  updated_at
FROM zatca_egs_chain
WHERE branch_id = ? AND egs_id = ?
FOR UPDATE;

-- name: ClaimZatcaEGSChain :execresult
UPDATE zatca_egs_chain
SET reservation_key = ?,
    reservation_token = ?,
    reserved_icv = last_icv + 1,
    reserved_previous_hash = previous_invoice_hash,
    reserved_at = CURRENT_TIMESTAMP,
    lease_expires_at = ?
WHERE branch_id = ?
  AND egs_id = ?
  AND (
    reservation_key IS NULL
    OR lease_expires_at IS NULL
    OR lease_expires_at <= CURRENT_TIMESTAMP
  );

-- name: CommitZatcaEGSChain :execresult
UPDATE zatca_egs_chain
SET last_icv = reserved_icv,
    previous_invoice_hash = ?,
    reservation_key = NULL,
    reservation_token = NULL,
    reserved_icv = NULL,
    reserved_previous_hash = NULL,
    reserved_at = NULL,
    lease_expires_at = NULL
WHERE branch_id = ?
  AND egs_id = ?
  AND reservation_key = ?
  AND reservation_token = ?
  AND reserved_icv = ?;

-- name: ReleaseZatcaEGSChain :execresult
UPDATE zatca_egs_chain
SET reservation_key = NULL,
    reservation_token = NULL,
    reserved_icv = NULL,
    reserved_previous_hash = NULL,
    reserved_at = NULL,
    lease_expires_at = NULL
WHERE branch_id = ?
  AND egs_id = ?
  AND reservation_key = ?
  AND reservation_token = ?;
