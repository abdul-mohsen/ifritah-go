-- name: ListSettings :many
SELECT setting_key, COALESCE(value, '') AS value
FROM settings
ORDER BY setting_key;

-- name: UpsertSetting :exec
INSERT INTO settings (setting_key, value, updated_by)
VALUES (?, ?, ?)
ON DUPLICATE KEY UPDATE value = VALUES(value), updated_by = VALUES(updated_by);
