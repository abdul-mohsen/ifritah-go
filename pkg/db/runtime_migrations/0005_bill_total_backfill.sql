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