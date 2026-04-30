-- name: Register :execresult
INSERT INTO user (company_id, username, password ) VALUES (?, ?, ?);

-- name: GetUserForLogin :one
SELECT id, password, role, is_active FROM user WHERE username = ? LIMIT 1;

-- name: GetUserAuthState :one
SELECT role, is_active FROM user WHERE id = ? LIMIT 1;

-- name: UpdateLastLogin :exec
UPDATE user SET last_login = NOW() WHERE id = ?;

-- name: InsertRefreshToken :exec
INSERT INTO refresh_token (user_id, token_hash, device_name, ip_address, expires_at)
VALUES (?, ?, ?, ?, ?);

-- name: GetActiveRefreshTokenID :one
SELECT id FROM refresh_token
WHERE token_hash = ? AND revoked = 0 AND expires_at > NOW()
LIMIT 1;

-- name: RotateRefreshToken :exec
UPDATE refresh_token SET token_hash = ?, expires_at = ? WHERE id = ?;

-- name: DeleteRefreshTokensForUser :exec
DELETE FROM refresh_token WHERE user_id = ?;

-- name: CountUsersByUsername :one
SELECT COUNT(*) FROM user WHERE username = ?;

-- name: CountUsersByEmail :one
SELECT COUNT(*) FROM user WHERE email = ?;

-- name: RegisterUser :execresult
INSERT INTO user (username, email, password, full_name, phone, role, is_active)
VALUES (?, ?, ?, ?, ?, 'employee', 1);

-- name: SeedUserPermission :exec
INSERT INTO user_permission (user_id, resource, can_view, can_add, can_edit, can_delete)
VALUES (?, ?, 1, 0, 0, 0);

-- name: GetUserIDByEmailActive :one
SELECT id FROM user WHERE email = ? AND is_active = 1;

-- name: InsertPasswordResetToken :exec
INSERT INTO password_reset_tokens (user_id, token, expires_at) VALUES (?, ?, ?);

-- name: GetActiveResetToken :one
SELECT id, user_id FROM password_reset_tokens
WHERE token = ? AND expires_at > NOW() AND used_at IS NULL
LIMIT 1;

-- name: MarkResetTokenUsed :exec
UPDATE password_reset_tokens SET used_at = NOW() WHERE id = ?;

-- name: UpdateUserPassword :execresult
UPDATE user SET password = ? WHERE id = ?;

-- name: DeleteSessionsForUser :exec
DELETE FROM sessions WHERE user_id = ?;

-- name: ListUsers :many
SELECT id, username, COALESCE(full_name,'') AS full_name, COALESCE(email,'') AS email,
       COALESCE(phone,'') AS phone, role, is_active, company_id,
       DATE_FORMAT(last_login, '%Y-%m-%dT%H:%i:%sZ') AS last_login
FROM user
ORDER BY id ASC;

-- name: GetUserAdmin :one
SELECT id, username, COALESCE(full_name,'') AS full_name, COALESCE(email,'') AS email,
       COALESCE(phone,'') AS phone, role, is_active, company_id,
       DATE_FORMAT(last_login, '%Y-%m-%dT%H:%i:%sZ') AS last_login
FROM user WHERE id = ?;

-- name: GetUserCompanyID :one
SELECT company_id FROM user WHERE id = ?;

-- name: GetUserRole :one
SELECT role FROM user WHERE id = ?;

-- name: CountActiveAdmins :one
SELECT COUNT(*) FROM user WHERE role = 'admin' AND is_active = 1;

-- name: CreateUserAdmin :execresult
INSERT INTO user (username, full_name, password, email, phone, company_id, is_active, role)
VALUES (
    sqlc.arg('username'),
    sqlc.arg('full_name'),
    sqlc.arg('password'),
    NULLIF(sqlc.arg('email'), ''),
    NULLIF(sqlc.arg('phone'), ''),
    sqlc.narg('company_id'),
    sqlc.arg('is_active'),
    sqlc.arg('role')
);

-- name: UpdateUserAdmin :execresult
UPDATE user SET
    full_name  = COALESCE(sqlc.narg('full_name'),  full_name),
    email      = COALESCE(sqlc.narg('email'),      email),
    phone      = COALESCE(sqlc.narg('phone'),      phone),
    role       = COALESCE(sqlc.narg('role'),       role),
    is_active  = COALESCE(sqlc.narg('is_active'),  is_active),
    company_id = COALESCE(sqlc.narg('company_id'), company_id)
WHERE id = sqlc.arg('id');

-- name: DeactivateUser :exec
UPDATE user SET is_active = 0 WHERE id = ?;

-- name: GetUserSelf :one
SELECT id, username, COALESCE(email,'') AS email,
       COALESCE(full_name,'') AS full_name,
       COALESCE(phone,'') AS phone,
       role, is_active
FROM user WHERE id = ?;

-- name: ListUserPermissions :many
SELECT resource, can_view, can_add, can_edit, can_delete
FROM user_permission WHERE user_id = ?;
