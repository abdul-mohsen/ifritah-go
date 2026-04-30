-- ============================================================================
-- 20260430_01_performance_indexes.sql
-- Performance indexes for hot query paths.
-- Idempotent: safe to re-run.
-- ============================================================================

DELIMITER $$

DROP PROCEDURE IF EXISTS add_index_if_missing $$
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
        SET @sql = p_ddl;
        PREPARE stmt FROM @sql;
        EXECUTE stmt;
        DEALLOCATE PREPARE stmt;
    END IF;
END $$

DELIMITER ;

-- ----------------------------------------------------------------------------
-- P1: must add now
-- ----------------------------------------------------------------------------

-- 1) user.username unique  (login lookups; prevent dup users)
CALL add_index_if_missing('user', 'uq_user_username',
    'ALTER TABLE `user` ADD UNIQUE KEY `uq_user_username` (`username`)');

-- 2) refresh_token.token_hash unique  (rotation correctness)
CALL add_index_if_missing('refresh_token', 'uq_rt_token_hash',
    'ALTER TABLE `refresh_token` ADD UNIQUE KEY `uq_rt_token_hash` (`token_hash`)');

-- 3) bill (merchant_id, state, effective_date)  (list & filter)
CALL add_index_if_missing('bill', 'idx_bill_merchant_state_date',
    'ALTER TABLE `bill` ADD INDEX `idx_bill_merchant_state_date` (`merchant_id`, `state`, `effective_date`)');

-- 4) bill (client_id, effective_date)  (client aging / credit-note joins)
CALL add_index_if_missing('bill', 'idx_bill_client_date',
    'ALTER TABLE `bill` ADD INDEX `idx_bill_client_date` (`client_id`, `effective_date`)');

-- 5) purchase_bill (supplier_id, effective_date)  (supplier report)
CALL add_index_if_missing('purchase_bill', 'idx_pb_supplier_date',
    'ALTER TABLE `purchase_bill` ADD INDEX `idx_pb_supplier_date` (`supplier_id`, `effective_date`)');

-- 6) purchase_bill_product.bill_id  (the SUM/GROUP BY join)
CALL add_index_if_missing('purchase_bill_product', 'idx_pbprod_bill',
    'ALTER TABLE `purchase_bill_product` ADD INDEX `idx_pbprod_bill` (`bill_id`)');

-- 7) product (store_id, is_deleted, quantity)  (low-stock dashboard)
CALL add_index_if_missing('product', 'idx_product_store_active_qty',
    'ALTER TABLE `product` ADD INDEX `idx_product_store_active_qty` (`store_id`, `is_deleted`, `quantity`)');

-- 8) supplier (company_id, is_deleted)  (tenant + soft-delete)
CALL add_index_if_missing('supplier', 'idx_supplier_company_active',
    'ALTER TABLE `supplier` ADD INDEX `idx_supplier_company_active` (`company_id`, `is_deleted`)');

-- 9) client (is_deleted, name) — client is a global table (no company_id);
--    pair the soft-delete flag with name for active-list lookups.
CALL add_index_if_missing('client', 'idx_client_active_name',
    'ALTER TABLE `client` ADD INDEX `idx_client_active_name` (`is_deleted`, `name`)');

-- 10) cash_voucher (merchant_id, effective_date, state)
CALL add_index_if_missing('cash_voucher', 'idx_cv_merchant_date_state',
    'ALTER TABLE `cash_voucher` ADD INDEX `idx_cv_merchant_date_state` (`merchant_id`, `effective_date`, `state`)');

-- 11) orders (store_id, status)
CALL add_index_if_missing('orders', 'idx_orders_store_status',
    'ALTER TABLE `orders` ADD INDEX `idx_orders_store_status` (`store_id`, `status`)');

-- ----------------------------------------------------------------------------
-- P2: should add
-- ----------------------------------------------------------------------------

-- 12) bill_payment.recorded_by
CALL add_index_if_missing('bill_payment', 'idx_billpay_recorded_by',
    'ALTER TABLE `bill_payment` ADD INDEX `idx_billpay_recorded_by` (`recorded_by`)');

-- 13) purchase_bill_payment.recorded_by
CALL add_index_if_missing('purchase_bill_payment', 'idx_pbpay_recorded_by',
    'ALTER TABLE `purchase_bill_payment` ADD INDEX `idx_pbpay_recorded_by` (`recorded_by`)');

-- 14) uploaded_files (uploaded_by, created_at)
CALL add_index_if_missing('uploaded_files', 'idx_uploaded_by_date',
    'ALTER TABLE `uploaded_files` ADD INDEX `idx_uploaded_by_date` (`uploaded_by`, `created_at`)');

DROP PROCEDURE IF EXISTS add_index_if_missing;
