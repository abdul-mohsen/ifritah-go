-- Indexes backing typed-field filter chips.
-- Idempotent via two helper procedures: drops legacy phone_digits
-- columns + indexes from earlier rounds, then creates the merged
-- index set on the raw columns.

DELIMITER $$

DROP PROCEDURE IF EXISTS _ifritah_drop_col$$
CREATE PROCEDURE _ifritah_drop_col(IN tbl VARCHAR(64), IN col VARCHAR(64))
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_schema = DATABASE() AND table_name = tbl AND column_name = col) THEN
        SET @s = CONCAT('ALTER TABLE `', tbl, '` DROP COLUMN `', col, '`');
        PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
    END IF;
END$$

DROP PROCEDURE IF EXISTS _ifritah_create_idx$$
CREATE PROCEDURE _ifritah_create_idx(IN tbl VARCHAR(64), IN idx VARCHAR(64), IN cols VARCHAR(255))
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.statistics
                   WHERE table_schema = DATABASE() AND table_name = tbl AND index_name = idx) THEN
        SET @s = CONCAT('CREATE INDEX `', idx, '` ON `', tbl, '` (', cols, ')');
        PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
    END IF;
END$$

DELIMITER ;

-- Drop legacy generated columns from earlier rounds.
CALL _ifritah_drop_col('bill',     'user_phone_digits');
CALL _ifritah_drop_col('client',   'phone_digits');
CALL _ifritah_drop_col('supplier', 'phone_digits');
CALL _ifritah_drop_col('branches', 'phone_digits');
CALL _ifritah_drop_col('user',     'phone_digits');

-- Create indexes on the raw columns.
CALL _ifritah_create_idx('bill',          'idx_bill_user_phone_number',           '`user_phone_number`');
CALL _ifritah_create_idx('bill',          'idx_bill_sequence_number',             '`sequence_number`');
CALL _ifritah_create_idx('client',        'idx_client_phone',                     '`phone`');
CALL _ifritah_create_idx('client',        'idx_client_vat_number',                '`vat_number`');
CALL _ifritah_create_idx('client',        'idx_client_commercial_registration',   '`commercial_registration`');
CALL _ifritah_create_idx('supplier',      'idx_supplier_phone_number',            '`phone_number`(64)');
CALL _ifritah_create_idx('supplier',      'idx_supplier_vat_number',              '`vat_number`(64)');
CALL _ifritah_create_idx('supplier',      'idx_supplier_commercial_registration', '`commercial_registration`');
CALL _ifritah_create_idx('purchase_bill', 'idx_pb_supplier_sequence_number',      '`supplier_sequence_number`');
CALL _ifritah_create_idx('branches',      'idx_branches_phone',                   '`phone`');
CALL _ifritah_create_idx('user',          'idx_user_phone',                       '`phone`');
CALL _ifritah_create_idx('user',          'idx_user_email',                       '`email`(64)');
CALL _ifritah_create_idx('cash_voucher',  'idx_cash_voucher_voucher_number',      '`voucher_number`');
CALL _ifritah_create_idx('articles',      'idx_articles_articleNumber',           '`articleNumber`(64)');
CALL _ifritah_create_idx('articleean',    'idx_articleean_eancode_legacy',        '`eancode`, `legacyArticleId`');

DROP PROCEDURE _ifritah_drop_col;
DROP PROCEDURE _ifritah_create_idx;
