-- Index product names by store so purchase-bill name resolution stays bounded
-- as each tenant's catalog grows. The guard keeps this migration replay-safe
-- on MySQL versions that do not support ADD INDEX IF NOT EXISTS.

SET @s := IF(EXISTS(
  SELECT 1
  FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 'product'
    AND index_name = 'idx_product_store_name'
),
  'SELECT 1',
  'ALTER TABLE `product` ADD INDEX `idx_product_store_name` (`store_id`, `name`)'
);
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;
