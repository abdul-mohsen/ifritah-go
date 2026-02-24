-- name: CreateBill :execresult
  insert into bill (effective_date, payment_due_date, state, sub_total, discount, vat, store_id, sequence_number, merchant_id, maintenance_cost, note, userName, buyer_id, user_phone_number)
  values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
