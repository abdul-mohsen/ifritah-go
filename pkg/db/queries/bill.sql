-- name: CreateBill :execresult
  insert into bill (effective_date, payment_due_date, state, sub_total, discount, vat, store_id, sequence_number, merchant_id, maintenance_cost, note, userName, buyer_id, user_phone_number)
  values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetAllBill :many
SELECT * from(
			SELECT bill.id as id, effective_date, payment_due_date, bill.state as state, discount, sequence_number, bill.user_phone_number, TRUE as bill_type, cn.state as credit_state, total, total_vat, total_before_vat
			FROM bill_totals as bill
			JOIN credit_note  cn on cn.bill_id = bill.id
			WHERE bill.user_phone_number like ?  and bill.state >= 0
			UNION
			SELECT bill.id as id, effective_date, payment_due_date, bill.state as state, discount, sequence_number, user_phone_number, TRUE as bill_type, 0 as credit_state, total, total_vat, total_before_vat from bill_totals as bill WHERE bill.user_phone_number like ? and bill. state >= 0
		) AS T ORDER BY id DESC LIMIT ? OFFSET ?;
