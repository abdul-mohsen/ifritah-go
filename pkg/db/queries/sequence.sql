-- name: AllocateBranchSequence :execresult
INSERT INTO branch_sequence (branch_id, scope, `last_value`)
VALUES (?, ?, LAST_INSERT_ID(1))
ON DUPLICATE KEY UPDATE `last_value` = LAST_INSERT_ID(`last_value` + 1);