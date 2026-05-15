-- 0005_branch_sequence_counters.sql
-- Per-branch monotonic sequence allocator. Idempotent.

SET @has_branch_sequence := (
  SELECT COUNT(*) FROM information_schema.tables
   WHERE table_schema = DATABASE()
     AND table_name = 'branch_sequence'
);
SET @sql := IF(@has_branch_sequence = 0,
  'CREATE TABLE `branch_sequence` (
    `branch_id`  INT UNSIGNED    NOT NULL,
    `scope`      VARCHAR(32)     NOT NULL,
    `last_value` BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `updated_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`branch_id`, `scope`),
    CONSTRAINT `fk_bsq_branch` FOREIGN KEY (`branch_id`)
      REFERENCES `branches` (`id`) ON DELETE CASCADE
  ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci',
  'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

UPDATE bill SET sequence_number = NULL WHERE sequence_number = 0;

INSERT IGNORE INTO `branch_sequence` (branch_id, scope, `last_value`)
SELECT branch_id, 'bill', COALESCE(MAX(sequence_number), 0)
  FROM bill
 WHERE branch_id IS NOT NULL
 GROUP BY branch_id;

-- Update using subquery instead of JOIN (for sqlc compatibility)
UPDATE `branch_sequence` bs SET bs.`last_value` = (
  SELECT GREATEST(bs.`last_value`, COALESCE(MAX(b.sequence_number), 0))
    FROM bill b
   WHERE b.branch_id = bs.branch_id
     AND bs.scope = 'bill'
)
 WHERE bs.scope = 'bill';

SET @has_uq := (
  SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE()
     AND table_name   = 'bill'
     AND index_name   = 'uq_bill_branch_seq'
);
SET @sql := IF(@has_uq = 0,
  'ALTER TABLE bill ADD UNIQUE KEY uq_bill_branch_seq (branch_id, sequence_number)',
  'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;
