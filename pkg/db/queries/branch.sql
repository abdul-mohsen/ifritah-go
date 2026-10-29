-- name: GetBranchByBillID :one
SELECT branches.id as branch_id from bill join branches on branch_id = branches.id where is_deleted = FALSE and bill.id = ? limit 1;


-- name: GetZatcaBranchConfig :one
SELECT
COALESCE(csr_org_identifier,'') as orgID, COALESCE(csr_org_unit,'') as orgUnit,
COALESCE(csr_org_name,'') as orgName, COALESCE(csr_country,'SA') as csrCountry,
COALESCE(csr_location,'') as csrLoc, COALESCE(business_category,'Supply activities') as bizCat,
COALESCE(seller_vat,'') as vat, COALESCE(seller_crn,'') as crn,
COALESCE(street,'') as street, COALESCE(building,'') as building, COALESCE(district,'') as district,
COALESCE(postal_code,'') as postal,
LENGTH(COALESCE(zatca_csr,'')) as csrLen,
LENGTH(COALESCE(zatca_production_username,'')) as prodLen,
zatca_registered_at,
zatca_status,
zatca_onboarded_at,
b.name,
COALESCE(config.onboard_state, 'not_started') AS onboard_state,
COALESCE(config.onboard_step, 'none') AS onboard_step,
COALESCE(config.onboard_attempt, 0) AS onboard_attempt,
COALESCE(config.onboard_retry_count, 0) AS onboard_retry_count,
config.onboard_error_code,
config.onboard_error,
config.onboard_queued_at,
config.onboard_started_at,
config.onboard_completed_at,
config.last_error,
config.last_attempt_at
FROM branch_zatca_config AS config
JOIN branches AS b ON b.id = config.branch_id
WHERE config.branch_id = ?
LIMIT 1;

-- name: GetBranchName :one
SELECT name FROM branches WHERE id = ?;

-- name: UpdateZatcaBranchConfig :exec
INSERT INTO branch_zatca_config
(branch_id, csr_org_identifier, csr_org_unit, csr_org_name,
  csr_country, csr_location, business_category,
  seller_vat, seller_crn, street, building, district, postal_code, zatca_status)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
csr_org_identifier = VALUES(csr_org_identifier),
csr_org_unit = VALUES(csr_org_unit),
csr_org_name = VALUES(csr_org_name),
csr_country = VALUES(csr_country),
csr_location = VALUES(csr_location),
business_category = VALUES(business_category),
seller_vat = VALUES(seller_vat),
seller_crn = VALUES(seller_crn),
street = VALUES(street),
building = VALUES(building),
district = VALUES(district),
postal_code = VALUES(postal_code),
zatca_status = VALUES(zatca_status);

-- name: QueueZatcaOnboarding :execresult
-- The state predicate makes the transition idempotent and prevents a second
-- active/done request from creating a duplicate attempt.
UPDATE branch_zatca_config
SET onboard_state = 'queued',
    onboard_step = 'queued',
    onboard_attempt = onboard_attempt + 1,
    onboard_retry_count = 0,
    onboard_error_code = NULL,
    onboard_error = NULL,
    last_error = NULL,
    onboard_queued_at = CURRENT_TIMESTAMP,
    onboard_started_at = NULL,
    onboard_completed_at = NULL,
    last_attempt_at = CURRENT_TIMESTAMP
WHERE branch_id = ?
  AND onboard_state IN ('not_started', 'failed');

-- name: CreateZatcaOnboardingAttempt :exec
INSERT INTO zatca_onboarding_attempt
  (branch_id, attempt_number, state, step, retry_count, queued_at)
VALUES (?, ?, 'queued', 'queued', 0, CURRENT_TIMESTAMP);

-- name: MarkZatcaOnboardingFailed :exec
UPDATE branch_zatca_config
SET onboard_state = 'failed',
    onboard_step = 'failed',
    onboard_retry_count = onboard_retry_count + 1,
    onboard_error_code = sqlc.arg('error_code'),
    onboard_error = sqlc.arg('error_message'),
    last_error = sqlc.arg('error_message'),
    last_attempt_at = CURRENT_TIMESTAMP
WHERE branch_id = sqlc.arg('branch_id')
  AND onboard_attempt = sqlc.arg('attempt_number');

-- name: MarkZatcaOnboardingAttemptFailed :exec
UPDATE zatca_onboarding_attempt
SET state = 'failed',
    step = 'failed',
    retry_count = retry_count + 1,
    error_code = sqlc.arg('error_code'),
    error_message = sqlc.arg('error_message'),
    completed_at = CURRENT_TIMESTAMP
WHERE branch_id = sqlc.arg('branch_id')
  AND attempt_number = sqlc.arg('attempt_number');

-- name: UpdateZatcaOnboardingProgress :exec
UPDATE branch_zatca_config
SET onboard_state = sqlc.arg('state'),
    onboard_step = sqlc.arg('step'),
    onboard_retry_count = sqlc.arg('retry_count'),
    onboard_error_code = sqlc.arg('error_code'),
    onboard_error = sqlc.arg('error_message'),
    last_error = sqlc.arg('error_message'),
    onboard_started_at = COALESCE(onboard_started_at, CURRENT_TIMESTAMP),
    last_attempt_at = CURRENT_TIMESTAMP
WHERE branch_id = sqlc.arg('branch_id')
  AND onboard_attempt = sqlc.arg('attempt_number');

-- name: UpdateZatcaOnboardingAttempt :exec
UPDATE zatca_onboarding_attempt
SET state = sqlc.arg('state'),
    step = sqlc.arg('step'),
    retry_count = sqlc.arg('retry_count'),
    error_code = sqlc.arg('error_code'),
    error_message = sqlc.arg('error_message'),
    started_at = COALESCE(started_at, CURRENT_TIMESTAMP)
WHERE branch_id = sqlc.arg('branch_id')
  AND attempt_number = sqlc.arg('attempt_number');

-- name: CompleteZatcaOnboarding :exec
UPDATE branch_zatca_config
SET onboard_state = 'done',
    onboard_step = 'done',
    onboard_error_code = NULL,
    onboard_error = NULL,
    last_error = NULL,
    onboard_completed_at = CURRENT_TIMESTAMP,
    zatca_onboarded_at = COALESCE(zatca_onboarded_at, CURRENT_TIMESTAMP),
    last_attempt_at = CURRENT_TIMESTAMP
WHERE branch_id = sqlc.arg('branch_id')
  AND onboard_attempt = sqlc.arg('attempt_number');

-- name: CompleteZatcaOnboardingAttempt :exec
UPDATE zatca_onboarding_attempt
SET state = 'done',
    step = 'done',
    completed_at = CURRENT_TIMESTAMP
WHERE branch_id = sqlc.arg('branch_id')
  AND attempt_number = sqlc.arg('attempt_number');

-- name: ListZatcaOnboardingAttempts :many
SELECT
  id,
  branch_id,
  attempt_number,
  state,
  step,
  retry_count,
  error_code,
  error_message,
  queued_at,
  started_at,
  completed_at,
  created_at,
  updated_at
FROM zatca_onboarding_attempt
WHERE branch_id = ?
ORDER BY attempt_number ASC;
