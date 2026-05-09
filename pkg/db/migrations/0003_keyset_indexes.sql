-- Keyset pagination indexes. MySQL 8.0 does not support the optional
-- IF NOT EXISTS clause here, so each index is guarded like the other migrations.

SET @s := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='bill' AND index_name='idx_bill_keyset')=0,
    'CREATE INDEX `idx_bill_keyset` ON `bill` (`effective_date` DESC, `id` DESC)', 'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='purchase_bill' AND index_name='idx_pb_keyset')=0,
    'CREATE INDEX `idx_pb_keyset` ON `purchase_bill` (`effective_date` DESC, `id` DESC)', 'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='cash_voucher' AND index_name='idx_cv_keyset')=0,
    'CREATE INDEX `idx_cv_keyset` ON `cash_voucher` (`effective_date` DESC, `id` DESC)', 'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='orders' AND index_name='idx_orders_keyset')=0,
    'CREATE INDEX `idx_orders_keyset` ON `orders` (`created_at` DESC, `id` DESC)', 'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='client' AND index_name='idx_client_keyset')=0,
    'CREATE INDEX `idx_client_keyset` ON `client` (`is_deleted`, `updated_at` DESC, `id` DESC)', 'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
