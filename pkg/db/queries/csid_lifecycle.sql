-- Canonical CSID lifecycle state. Every query is tenant, branch, and EGS
-- sequence scoped; the EGS sequence remains the source of EGS/VAT identity.

-- name: GetZatcaEGSCredential :one
SELECT
  c.id,
  c.tenant_id,
  c.branch_id,
  c.egs_sequence_id,
  s.vat_number,
  s.egs_id,
  c.credential_type,
  c.certificate_serial,
  c.certificate_expires_at,
  c.status,
  c.renewal_state,
  c.revocation_state,
  c.renewal_requested_at,
  c.renewed_at,
  c.revocation_requested_at,
  c.revoked_at,
  c.revocation_reason,
  c.last_error,
  c.created_at,
  c.updated_at
FROM zatca_egs_credential AS c
JOIN egs_sequence AS s
  ON s.tenant_id = c.tenant_id
 AND s.branch_id = c.branch_id
 AND s.egs_sequence_id = c.egs_sequence_id
WHERE c.tenant_id = sqlc.arg('tenant_id')
  AND c.branch_id = sqlc.arg('branch_id')
  AND c.egs_sequence_id = sqlc.arg('egs_sequence_id')
  AND c.credential_type = sqlc.arg('credential_type')
LIMIT 1;

-- name: ListZatcaEGSCredentialsByBranch :many
SELECT
  c.id,
  c.tenant_id,
  c.branch_id,
  c.egs_sequence_id,
  s.vat_number,
  s.egs_id,
  c.credential_type,
  c.certificate_serial,
  c.certificate_expires_at,
  c.status,
  c.renewal_state,
  c.renewal_requested_at,
  c.renewed_at,
  c.revocation_state,
  c.revocation_requested_at,
  c.revoked_at,
  c.revocation_reason,
  c.last_error,
  c.created_at,
  c.updated_at
FROM zatca_egs_credential AS c
JOIN egs_sequence AS s
  ON s.tenant_id = c.tenant_id
 AND s.branch_id = c.branch_id
 AND s.egs_sequence_id = c.egs_sequence_id
WHERE c.tenant_id = sqlc.arg('tenant_id')
  AND c.branch_id = sqlc.arg('branch_id')
ORDER BY c.egs_sequence_id ASC, c.credential_type ASC;

-- name: ListZatcaEGSCredentialsForLifecycle :many
SELECT
  c.id,
  c.tenant_id,
  c.branch_id,
  c.egs_sequence_id,
  s.vat_number,
  s.egs_id,
  c.credential_type,
  c.certificate_serial,
  c.certificate_expires_at,
  c.status,
  c.renewal_state,
  c.revocation_state,
  c.renewal_requested_at,
  c.renewed_at,
  c.revocation_requested_at,
  c.revoked_at,
  c.revocation_reason,
  c.last_error,
  c.created_at,
  c.updated_at
FROM zatca_egs_credential AS c
JOIN egs_sequence AS s
  ON s.tenant_id = c.tenant_id
 AND s.branch_id = c.branch_id
 AND s.egs_sequence_id = c.egs_sequence_id
WHERE c.tenant_id = sqlc.arg('tenant_id')
  AND (
    c.status = 'pending'
    OR c.renewal_state = 'pending'
    OR c.revocation_state = 'pending'
    OR (
      c.status = 'active'
      AND c.certificate_expires_at IS NOT NULL
      AND c.certificate_expires_at <= sqlc.arg('expires_before')
    )
  )
ORDER BY c.updated_at ASC, c.id ASC
LIMIT ?;

-- name: CreateZatcaEGSCredential :execresult
INSERT INTO zatca_egs_credential (
  tenant_id,
  branch_id,
  egs_sequence_id,
  credential_type,
  status,
  renewal_state,
  revocation_state
)
VALUES (
  sqlc.arg('tenant_id'),
  sqlc.arg('branch_id'),
  sqlc.arg('egs_sequence_id'),
  sqlc.arg('credential_type'),
  'pending',
  'none',
  'none'
);

-- name: RequestZatcaEGSCredentialRenewal :execresult
UPDATE zatca_egs_credential
SET
  status = 'pending',
  renewal_state = 'pending',
  renewal_requested_at = sqlc.arg('requested_at'),
  last_error = NULL
WHERE tenant_id = sqlc.arg('tenant_id')
  AND branch_id = sqlc.arg('branch_id')
  AND egs_sequence_id = sqlc.arg('egs_sequence_id')
  AND credential_type = sqlc.arg('credential_type')
  AND status IN ('pending','active','expired')
  AND revocation_state = 'none';

-- name: RecordZatcaEGSCredentialIssued :execresult
UPDATE zatca_egs_credential
SET
  certificate_serial = sqlc.arg('certificate_serial'),
  certificate_expires_at = sqlc.arg('certificate_expires_at'),
  status = 'active',
  renewal_state = 'completed',
  renewed_at = sqlc.arg('issued_at'),
  revocation_state = 'none',
  revocation_requested_at = NULL,
  last_error = NULL
WHERE tenant_id = sqlc.arg('tenant_id')
  AND branch_id = sqlc.arg('branch_id')
  AND egs_sequence_id = sqlc.arg('egs_sequence_id')
  AND credential_type = sqlc.arg('credential_type')
  AND status IN ('pending','active','expired')
  AND revocation_state = 'none';

-- name: MarkZatcaEGSCredentialRenewalFailed :execresult
UPDATE zatca_egs_credential
SET
  renewal_state = 'failed',
  last_error = sqlc.arg('last_error')
WHERE tenant_id = sqlc.arg('tenant_id')
  AND branch_id = sqlc.arg('branch_id')
  AND egs_sequence_id = sqlc.arg('egs_sequence_id')
  AND credential_type = sqlc.arg('credential_type')
  AND status = 'pending'
  AND revocation_state = 'none';

-- name: MarkZatcaEGSCredentialExpired :execresult
UPDATE zatca_egs_credential
SET
  status = 'expired',
  last_error = NULL
WHERE tenant_id = sqlc.arg('tenant_id')
  AND branch_id = sqlc.arg('branch_id')
  AND egs_sequence_id = sqlc.arg('egs_sequence_id')
  AND credential_type = sqlc.arg('credential_type')
  AND status = 'active'
  AND revocation_state = 'none'
  AND certificate_expires_at IS NOT NULL
  AND certificate_expires_at <= sqlc.arg('as_of');

-- name: RequestZatcaEGSCredentialRevocation :execresult
UPDATE zatca_egs_credential
SET
  revocation_state = 'pending',
  revocation_requested_at = sqlc.arg('requested_at'),
  last_error = NULL
WHERE tenant_id = sqlc.arg('tenant_id')
  AND branch_id = sqlc.arg('branch_id')
  AND egs_sequence_id = sqlc.arg('egs_sequence_id')
  AND credential_type = sqlc.arg('credential_type')
  AND status IN ('pending','active','expired')
  AND revocation_state = 'none';

-- name: MarkZatcaEGSCredentialRevoked :execresult
UPDATE zatca_egs_credential
SET
  status = 'revoked',
  revocation_state = 'revoked',
  revoked_at = sqlc.arg('revoked_at'),
  revocation_reason = sqlc.arg('reason'),
  last_error = NULL
WHERE tenant_id = sqlc.arg('tenant_id')
  AND branch_id = sqlc.arg('branch_id')
  AND egs_sequence_id = sqlc.arg('egs_sequence_id')
  AND credential_type = sqlc.arg('credential_type')
  AND status <> 'revoked'
  AND revocation_state IN ('none','pending');
