-- Persist cost and shelf metadata for both inventory-linked and manual
-- purchase-bill lines. The migration is safe to apply repeatedly.

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'purchase_bill_product'
     AND column_name = 'cost_price') = 0,
  'ALTER TABLE `purchase_bill_product` ADD COLUMN `cost_price` DECIMAL(12,2) NOT NULL DEFAULT 0.00 AFTER `price`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE()
     AND table_name = 'purchase_bill_product'
     AND column_name = 'shelf_number') = 0,
  'ALTER TABLE `purchase_bill_product` ADD COLUMN `shelf_number` VARCHAR(45) NULL AFTER `name`',
  'DO 0'
);
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
