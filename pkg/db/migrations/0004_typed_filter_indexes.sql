-- Indexes backing typed-field filter chips.
-- Idempotent: drops legacy phone_digits columns from prior rounds,
-- then creates the merged index set on raw columns.

-- Drop legacy generated columns (PREPARE pattern: MySQL 8 has no DROP COLUMN IF EXISTS).
SET @s := IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='bill'     AND column_name='user_phone_digits')>0, 'ALTER TABLE `bill` DROP COLUMN `user_phone_digits`',     'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
SET @s := IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='client'   AND column_name='phone_digits')>0,      'ALTER TABLE `client` DROP COLUMN `phone_digits`',          'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
SET @s := IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='supplier' AND column_name='phone_digits')>0,      'ALTER TABLE `supplier` DROP COLUMN `phone_digits`',        'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
SET @s := IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='branches' AND column_name='phone_digits')>0,      'ALTER TABLE `branches` DROP COLUMN `phone_digits`',        'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
SET @s := IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='user'     AND column_name='phone_digits')>0,      'ALTER TABLE `user` DROP COLUMN `phone_digits`',            'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

-- Indexes (idempotent: skip when an index of the same name already exists).
SET @s := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='bill'          AND index_name='idx_bill_user_phone_number')=0,           'CREATE INDEX `idx_bill_user_phone_number` ON `bill` (`user_phone_number`)',                       'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
SET @s := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='bill'          AND index_name='idx_bill_sequence_number')=0,             'CREATE INDEX `idx_bill_sequence_number` ON `bill` (`sequence_number`)',                           'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
SET @s := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='client'        AND index_name='idx_client_phone')=0,                     'CREATE INDEX `idx_client_phone` ON `client` (`phone`)',                                           'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
SET @s := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='client'        AND index_name='idx_client_vat_number')=0,                'CREATE INDEX `idx_client_vat_number` ON `client` (`vat_number`)',                                 'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
SET @s := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='client'        AND index_name='idx_client_commercial_registration')=0,   'CREATE INDEX `idx_client_commercial_registration` ON `client` (`commercial_registration`)',       'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
SET @s := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='supplier'      AND index_name='idx_supplier_phone_number')=0,            'CREATE INDEX `idx_supplier_phone_number` ON `supplier` (`phone_number`(64))',                     'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
SET @s := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='supplier'      AND index_name='idx_supplier_vat_number')=0,              'CREATE INDEX `idx_supplier_vat_number` ON `supplier` (`vat_number`(64))',                         'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
SET @s := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='supplier'      AND index_name='idx_supplier_commercial_registration')=0, 'CREATE INDEX `idx_supplier_commercial_registration` ON `supplier` (`commercial_registration`)',   'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
SET @s := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='purchase_bill' AND index_name='idx_pb_supplier_sequence_number')=0,      'CREATE INDEX `idx_pb_supplier_sequence_number` ON `purchase_bill` (`supplier_sequence_number`)', 'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
SET @s := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='branches'      AND index_name='idx_branches_phone')=0,                   'CREATE INDEX `idx_branches_phone` ON `branches` (`phone`)',                                       'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
SET @s := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='user'          AND index_name='idx_user_phone')=0,                       'CREATE INDEX `idx_user_phone` ON `user` (`phone`)',                                               'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
SET @s := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='user'          AND index_name='idx_user_email')=0,                       'CREATE INDEX `idx_user_email` ON `user` (`email`(64))',                                           'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
SET @s := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='cash_voucher'  AND index_name='idx_cash_voucher_voucher_number')=0,      'CREATE INDEX `idx_cash_voucher_voucher_number` ON `cash_voucher` (`voucher_number`)',             'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
SET @s := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='articles'      AND index_name='idx_articles_articleNumber')=0,           'CREATE INDEX `idx_articles_articleNumber` ON `articles` (`articleNumber`(64))',                   'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
SET @s := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='articleean'    AND index_name='idx_articleean_eancode_legacy')=0,        'CREATE INDEX `idx_articleean_eancode_legacy` ON `articleean` (`eancode`, `legacyArticleId`)',     'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
