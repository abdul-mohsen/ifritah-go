-- Migration: add columns required for user management API
--   - is_deleted: enables soft-delete (DELETE /api/v2/users/:id)
--   - manager_id: optional reporting hierarchy (employee -> manager)
-- Idempotent: re-applying does nothing.

DROP PROCEDURE IF EXISTS add_column_if_missing;
DELIMITER $$
CREATE PROCEDURE add_column_if_missing(
    IN p_table  VARCHAR(64),
    IN p_column VARCHAR(64),
    IN p_ddl    TEXT
)
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = DATABASE()
          AND table_name   = p_table
          AND column_name  = p_column
    ) THEN
        SET @sql = CONCAT('ALTER TABLE `', p_table, '` ADD COLUMN ', p_ddl);
        PREPARE stmt FROM @sql;
        EXECUTE stmt;
        DEALLOCATE PREPARE stmt;
    END IF;
END$$
DELIMITER ;

CALL add_column_if_missing('user', 'is_deleted',
    '`is_deleted` TINYINT(1) NOT NULL DEFAULT 0');

CALL add_column_if_missing('user', 'manager_id',
    '`manager_id` INT NULL DEFAULT NULL');

DROP PROCEDURE IF EXISTS add_column_if_missing;

-- Index to help admin user-list filters
DROP PROCEDURE IF EXISTS add_index_if_missing;
DELIMITER $$
CREATE PROCEDURE add_index_if_missing(
    IN p_table VARCHAR(64),
    IN p_index VARCHAR(64),
    IN p_ddl   TEXT
)
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.statistics
        WHERE table_schema = DATABASE()
          AND table_name   = p_table
          AND index_name   = p_index
    ) THEN
        SET @sql = CONCAT('ALTER TABLE `', p_table, '` ADD ', p_ddl);
        PREPARE stmt FROM @sql;
        EXECUTE stmt;
        DEALLOCATE PREPARE stmt;
    END IF;
END$$
DELIMITER ;

CALL add_index_if_missing('user', 'idx_user_role_active',
    'INDEX `idx_user_role_active` (`role`, `is_active`, `is_deleted`)');

DROP PROCEDURE IF EXISTS add_index_if_missing;
