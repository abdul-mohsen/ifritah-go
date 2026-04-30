-- ============================================================================
-- 20260430_01_performance_indexes.sql
-- Performance indexes for hot query paths.
-- Idempotent: safe to re-run.
-- ============================================================================

DROP TEMPORARY TABLE IF EXISTS _idx_todo;
CREATE TEMPORARY TABLE _idx_todo (
    t VARCHAR(64) NOT NULL,
    n VARCHAR(64) NOT NULL,
    d TEXT        NOT NULL
);

INSERT INTO _idx_todo (t, n, d) VALUES
    -- P1: must add now
    ('user',                  'uq_user_username',             'ALTER TABLE `user` ADD UNIQUE KEY `uq_user_username` (`username`)'),
    ('refresh_token',         'uq_rt_token_hash',             'ALTER TABLE `refresh_token` ADD UNIQUE KEY `uq_rt_token_hash` (`token_hash`)'),
    ('bill',                  'idx_bill_merchant_state_date', 'ALTER TABLE `bill` ADD INDEX `idx_bill_merchant_state_date` (`merchant_id`, `state`, `effective_date`)'),
    ('bill',                  'idx_bill_client_date',         'ALTER TABLE `bill` ADD INDEX `idx_bill_client_date` (`client_id`, `effective_date`)'),
    ('purchase_bill',         'idx_pb_supplier_date',         'ALTER TABLE `purchase_bill` ADD INDEX `idx_pb_supplier_date` (`supplier_id`, `effective_date`)'),
    ('purchase_bill_product', 'idx_pbprod_bill',              'ALTER TABLE `purchase_bill_product` ADD INDEX `idx_pbprod_bill` (`bill_id`)'),
    ('product',               'idx_product_store_active_qty', 'ALTER TABLE `product` ADD INDEX `idx_product_store_active_qty` (`store_id`, `is_deleted`, `quantity`)'),
    ('supplier',              'idx_supplier_company_active',  'ALTER TABLE `supplier` ADD INDEX `idx_supplier_company_active` (`company_id`, `is_deleted`)'),
    ('client',                'idx_client_active_name',       'ALTER TABLE `client` ADD INDEX `idx_client_active_name` (`is_deleted`, `name`)'),
    ('cash_voucher',          'idx_cv_merchant_date_state',   'ALTER TABLE `cash_voucher` ADD INDEX `idx_cv_merchant_date_state` (`merchant_id`, `effective_date`, `state`)'),
    ('orders',                'idx_orders_store_status',      'ALTER TABLE `orders` ADD INDEX `idx_orders_store_status` (`store_id`, `status`)'),
    -- P2: should add
    ('bill_payment',          'idx_billpay_recorded_by',      'ALTER TABLE `bill_payment` ADD INDEX `idx_billpay_recorded_by` (`recorded_by`)'),
    ('purchase_bill_payment', 'idx_pbpay_recorded_by',        'ALTER TABLE `purchase_bill_payment` ADD INDEX `idx_pbpay_recorded_by` (`recorded_by`)'),
    ('uploaded_files',        'idx_uploaded_by_date',         'ALTER TABLE `uploaded_files` ADD INDEX `idx_uploaded_by_date` (`uploaded_by`, `created_at`)');

DELIMITER $$

DROP PROCEDURE IF EXISTS apply_index_todo $$
CREATE PROCEDURE apply_index_todo()
BEGIN
    DECLARE done INT DEFAULT 0;
    DECLARE v_t VARCHAR(64);
    DECLARE v_n VARCHAR(64);
    DECLARE v_d TEXT;
    DECLARE cur CURSOR FOR SELECT t, n, d FROM _idx_todo;
    DECLARE CONTINUE HANDLER FOR NOT FOUND SET done = 1;

    OPEN cur;
    read_loop: LOOP
        FETCH cur INTO v_t, v_n, v_d;
        IF done THEN
            LEAVE read_loop;
        END IF;
        IF NOT EXISTS (
            SELECT 1 FROM information_schema.statistics
            WHERE table_schema = DATABASE()
              AND table_name   = v_t
              AND index_name   = v_n
        ) THEN
            SET @sql = v_d;
            PREPARE stmt FROM @sql;
            EXECUTE stmt;
            DEALLOCATE PREPARE stmt;
        END IF;
    END LOOP;
    CLOSE cur;
END $$

DELIMITER ;

CALL apply_index_todo();

DROP PROCEDURE IF EXISTS apply_index_todo;
DROP TEMPORARY TABLE IF EXISTS _idx_todo;
