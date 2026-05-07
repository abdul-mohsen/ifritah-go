-- Add VARCHAR mirror columns for numeric sequence_number fields used in
-- prefix-LIKE typed-filter queries. sqlc's MySQL parser cannot infer
-- *string from `CAST(numeric AS CHAR) LIKE narg` — it sees the underlying
-- column type and emits *uint64. Adding a VIRTUAL generated VARCHAR column
-- gives sqlc a column with the right type without changing the source data
-- or requiring per-query CAST gymnastics.
--
-- VIRTUAL = computed on read, no storage cost. Indexed so prefix LIKEs stay
-- fast.

SET @s := IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='bill'          AND column_name='sequence_number_str')=0,
            'ALTER TABLE `bill` ADD COLUMN `sequence_number_str` VARCHAR(32) GENERATED ALWAYS AS (CAST(`sequence_number` AS CHAR)) VIRTUAL', 'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='purchase_bill' AND column_name='sequence_number_str')=0,
            'ALTER TABLE `purchase_bill` ADD COLUMN `sequence_number_str` VARCHAR(32) GENERATED ALWAYS AS (CAST(`sequence_number` AS CHAR)) VIRTUAL', 'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='purchase_bill' AND column_name='supplier_sequence_number_str')=0,
            'ALTER TABLE `purchase_bill` ADD COLUMN `supplier_sequence_number_str` VARCHAR(64) GENERATED ALWAYS AS (CAST(`supplier_sequence_number` AS CHAR)) VIRTUAL', 'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='bill'          AND index_name='idx_bill_sequence_number_str')=0,
            'CREATE INDEX `idx_bill_sequence_number_str` ON `bill` (`sequence_number_str`)', 'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='purchase_bill' AND index_name='idx_pb_sequence_number_str')=0,
            'CREATE INDEX `idx_pb_sequence_number_str` ON `purchase_bill` (`sequence_number_str`)', 'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

SET @s := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='purchase_bill' AND index_name='idx_pb_supplier_sequence_number_str')=0,
            'CREATE INDEX `idx_pb_supplier_sequence_number_str` ON `purchase_bill` (`supplier_sequence_number_str`)', 'DO 0');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;
