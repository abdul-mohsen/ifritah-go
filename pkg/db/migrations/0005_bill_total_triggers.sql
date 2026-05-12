DROP TRIGGER IF EXISTS bill_product_after_insert_totals;
DROP TRIGGER IF EXISTS bill_product_after_update_totals;
DROP TRIGGER IF EXISTS bill_product_after_delete_totals;
DROP TRIGGER IF EXISTS purchase_bill_product_after_insert_totals;
DROP TRIGGER IF EXISTS purchase_bill_product_after_update_totals;
DROP TRIGGER IF EXISTS purchase_bill_product_after_delete_totals;

UPDATE bill
SET
  total_before_vat = COALESCE((SELECT SUM(total_before_vat) FROM bill_product WHERE bill_product.bill_id = bill.id), 0),
  total_vat = COALESCE((SELECT SUM(vat_total) FROM bill_product WHERE bill_product.bill_id = bill.id), 0),
  total = COALESCE((SELECT SUM(total_including_vat) FROM bill_product WHERE bill_product.bill_id = bill.id), 0),
  discount_amount = COALESCE((SELECT SUM(discount) FROM bill_product WHERE bill_product.bill_id = bill.id), 0);

UPDATE purchase_bill
SET
  total_before_vat = COALESCE((SELECT SUM(total_before_vat) FROM purchase_bill_product WHERE purchase_bill_product.bill_id = purchase_bill.id), 0),
  total_vat = COALESCE((SELECT SUM(vat_total) FROM purchase_bill_product WHERE purchase_bill_product.bill_id = purchase_bill.id), 0),
  total = COALESCE((SELECT SUM(total_including_vat) FROM purchase_bill_product WHERE purchase_bill_product.bill_id = purchase_bill.id), 0);

CREATE TRIGGER bill_product_after_insert_totals
AFTER INSERT ON bill_product
FOR EACH ROW
UPDATE bill
SET
  total_before_vat = total_before_vat + COALESCE(NEW.total_before_vat, 0),
  total_vat = total_vat + COALESCE(NEW.vat_total, 0),
  total = total + COALESCE(NEW.total_including_vat, 0),
  discount_amount = discount_amount + COALESCE(NEW.discount, 0)
WHERE id = NEW.bill_id;

CREATE TRIGGER bill_product_after_update_totals
AFTER UPDATE ON bill_product
FOR EACH ROW
UPDATE bill
SET
  total_before_vat = total_before_vat
    - CASE WHEN id = OLD.bill_id THEN COALESCE(OLD.total_before_vat, 0) ELSE 0 END
    + CASE WHEN id = NEW.bill_id THEN COALESCE(NEW.total_before_vat, 0) ELSE 0 END,
  total_vat = total_vat
    - CASE WHEN id = OLD.bill_id THEN COALESCE(OLD.vat_total, 0) ELSE 0 END
    + CASE WHEN id = NEW.bill_id THEN COALESCE(NEW.vat_total, 0) ELSE 0 END,
  total = total
    - CASE WHEN id = OLD.bill_id THEN COALESCE(OLD.total_including_vat, 0) ELSE 0 END
    + CASE WHEN id = NEW.bill_id THEN COALESCE(NEW.total_including_vat, 0) ELSE 0 END,
  discount_amount = discount_amount
    - CASE WHEN id = OLD.bill_id THEN COALESCE(OLD.discount, 0) ELSE 0 END
    + CASE WHEN id = NEW.bill_id THEN COALESCE(NEW.discount, 0) ELSE 0 END
WHERE id IN (OLD.bill_id, NEW.bill_id);

CREATE TRIGGER bill_product_after_delete_totals
AFTER DELETE ON bill_product
FOR EACH ROW
UPDATE bill
SET
  total_before_vat = total_before_vat - COALESCE(OLD.total_before_vat, 0),
  total_vat = total_vat - COALESCE(OLD.vat_total, 0),
  total = total - COALESCE(OLD.total_including_vat, 0),
  discount_amount = discount_amount - COALESCE(OLD.discount, 0)
WHERE id = OLD.bill_id;

CREATE TRIGGER purchase_bill_product_after_insert_totals
AFTER INSERT ON purchase_bill_product
FOR EACH ROW
UPDATE purchase_bill
SET
  total_before_vat = total_before_vat + COALESCE(NEW.total_before_vat, 0),
  total_vat = total_vat + COALESCE(NEW.vat_total, 0),
  total = total + COALESCE(NEW.total_including_vat, 0)
WHERE id = NEW.bill_id;

CREATE TRIGGER purchase_bill_product_after_update_totals
AFTER UPDATE ON purchase_bill_product
FOR EACH ROW
UPDATE purchase_bill
SET
  total_before_vat = total_before_vat
    - CASE WHEN id = OLD.bill_id THEN COALESCE(OLD.total_before_vat, 0) ELSE 0 END
    + CASE WHEN id = NEW.bill_id THEN COALESCE(NEW.total_before_vat, 0) ELSE 0 END,
  total_vat = total_vat
    - CASE WHEN id = OLD.bill_id THEN COALESCE(OLD.vat_total, 0) ELSE 0 END
    + CASE WHEN id = NEW.bill_id THEN COALESCE(NEW.vat_total, 0) ELSE 0 END,
  total = total
    - CASE WHEN id = OLD.bill_id THEN COALESCE(OLD.total_including_vat, 0) ELSE 0 END
    + CASE WHEN id = NEW.bill_id THEN COALESCE(NEW.total_including_vat, 0) ELSE 0 END
WHERE id IN (OLD.bill_id, NEW.bill_id);

CREATE TRIGGER purchase_bill_product_after_delete_totals
AFTER DELETE ON purchase_bill_product
FOR EACH ROW
UPDATE purchase_bill
SET
  total_before_vat = total_before_vat - COALESCE(OLD.total_before_vat, 0),
  total_vat = total_vat - COALESCE(OLD.vat_total, 0),
  total = total - COALESCE(OLD.total_including_vat, 0)
WHERE id = OLD.bill_id;