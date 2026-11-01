-- +goose Up
-- Bill numbers come from branch_sequence now, one counter per branch (scope 'bill'), not from
-- MAX(sequence_number)+1. Each branch starts at the highest number it already has, so the next sale
-- continues its series and cannot take an existing number (uq_bill_branch_seq). A branch without bills
-- gets its row on its first sale, and that first number is 1.
INSERT INTO branch_sequence (branch_id, scope, `last_value`)
SELECT branch_id, 'bill', MAX(sequence_number)
FROM bill
WHERE branch_id IS NOT NULL AND sequence_number IS NOT NULL
GROUP BY branch_id
ON DUPLICATE KEY UPDATE `last_value` = GREATEST(`last_value`, VALUES(`last_value`));

-- +goose Down
DELETE FROM branch_sequence WHERE scope = 'bill';
