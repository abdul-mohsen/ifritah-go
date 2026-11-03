package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/identity"
	"ifritah/web-service-gin/pkg/model"
)

// A sale whose request id was already used for other content, or for another bill, is answered with this code.
const (
	requestConflictCode    = "request_conflict"
	requestConflictMessage = "this sale request id was already used for a different sale or bill"
)

// AddBill saves a new sale or draft and answers with the bill id.
func (h *handler) AddBill(c *gin.Context) {
	request := newSaleRequest()
	if err := c.BindJSON(&request); err != nil {
		log.Printf("AddBill: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}
	billID, ok := h.saveSale(c, &request, nil)
	if !ok {
		return
	}
	c.JSON(http.StatusCreated, billID)
}

// SubmitDraftBill finalizes a draft with the content of the request.
func (h *handler) SubmitDraftBill(c *gin.Context) {
	billID, err := parseBillID(c.Param("id"))
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	request := newSaleRequest()
	if err := c.BindJSON(&request); err != nil {
		log.Printf("SubmitDraftBill: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}
	if _, ok := h.saveSale(c, &request, &billID); !ok {
		return
	}
	c.Status(http.StatusCreated)
}

func newSaleRequest() model.AddBillRequest {
	return model.AddBillRequest{
		State:         1,
		PaymentMethod: 0,
		PaidAmount:    decimal.NewFromInt(0),
	}
}

// saveSale writes a sale in one transaction. A nil draftID creates a bill, and a draft id finalizes that draft. It returns the
// bill id, and on failure it has already written the error response.
func (h *handler) saveSale(c *gin.Context, request *model.AddBillRequest, draftID *uint64) (uint64, bool) {
	ident, err := finalSaleIdentity(request)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return 0, false
	}
	if err := paidAmountError(request.PaidAmount); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return 0, false
	}
	if err := verifyFinalSale(request); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return 0, false
	}
	userSession := GetSessionInfo(c)
	if !h.checkBillStoreAccess(c, request.StoreId) {
		return 0, false
	}
	if !h.checkStoreBelongsToBranch(c, request.StoreId, request.BranchID) {
		return 0, false
	}

	tx, err := h.DB.Begin()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return 0, false
	}
	defer tx.Rollback()
	return h.writeSale(c, tx, request, ident, draftID, int32(userSession.id))
}

// writeSale is the transaction of a sale. A sale whose request id is already recorded writes nothing: the same content for the
// same bill gets the original result, and anything else is refused.
func (h *handler) writeSale(c *gin.Context, tx *sql.Tx, request *model.AddBillRequest, ident billIdentity, draftID *uint64, userID int32) (uint64, bool) {
	qtx := h.queries.WithTx(tx)
	if draftID == nil {
		original, found, ok := requestRecord(c, qtx, ident)
		if !ok {
			return 0, false
		}
		if found {
			return answerRecordedRequest(c, original, ident, nil)
		}
	}

	billID, final, ok := openSaleBill(c, qtx, request, draftID, userID)
	if !ok {
		return 0, false
	}
	if final {
		return answerFinalDraft(c, qtx, ident, draftID)
	}
	number, sequence, ok := numberSale(c, qtx, request)
	if !ok {
		return 0, false
	}
	if err := updateSaleBill(c, qtx, request, billID, sequence, ident, userID); err != nil {
		if !IsDuplicate(err) {
			log.Printf("sale update: %v", err)
			c.AbortWithError(http.StatusInternalServerError, err)
			return 0, false
		}
		// A concurrent request recorded this request id after the check above. The transaction is rolled back, and the
		// record is read again outside it, where the committed row is visible.
		_ = tx.Rollback()
		return answerLostRace(c, h.queries, ident, draftID)
	}
	sale := finalSale{billID: billID, number: number, identity: ident}
	if !h.finishSale(c, tx, qtx, request, sale, draftID, userID) {
		return 0, false
	}
	return billID, true
}

// finishSale writes what follows the bill row. A draft's old lines are removed first. Then the sale's lines, totals, stock
// movements and ZATCA event are written, and the transaction commits.
func (h *handler) finishSale(c *gin.Context, tx *sql.Tx, qtx *db.Queries, request *model.AddBillRequest, sale finalSale, draftID *uint64, userID int32) bool {
	if draftID != nil {
		if err := qtx.DeleteProductToBill(c.Request.Context(), sale.billID); err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return false
		}
	}
	if !h.completeSale(c, qtx, tx, request, sale, userID) {
		return false
	}
	if !recordSalePayment(c, qtx, request, sale.billID, draftID, userID) {
		return false
	}
	if err := tx.Commit(); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return false
	}
	return true
}

// saleRefusal is a sale that the sale path refuses. Its code names the rule, for the import check to report, and its message is the
// answer of the sale path.
type saleRefusal struct {
	code    string
	message string
}

func (r *saleRefusal) Error() string {
	return r.message
}

func refuse(code, message string) error {
	return &saleRefusal{code: code, message: message}
}

// verifyFinalSale checks a final sale before anything is numbered or written, so a bill that cannot be reported is refused while the
// cashier can still fix it. These are the checks the sale can make from its own request. The branch's seller data is checked by the
// daemon before it books an invoice number. A quantity and a price are checked at the scale that bill_product stores them, three and
// two decimals, because the database refuses a line that would be stored as zero.
func verifyFinalSale(request *model.AddBillRequest) error {
	if request.State <= 0 {
		return nil
	}
	if len(request.Products)+len(request.ManualProducts) == 0 && !request.MaintenanceCost.IsPositive() {
		return refuse("no_lines", "a final sale needs at least one line")
	}
	if request.MaintenanceCost.IsPositive() && !request.MaintenanceCost.Round(2).IsPositive() {
		return refuse("line_price", "maintenance_cost must be at least 0.01")
	}
	for i, line := range slices.Concat(request.Products, request.ManualProducts) {
		switch {
		case !line.Quantity.Round(3).IsPositive():
			return refuse("line_quantity", fmt.Sprintf("line %d: quantity must be positive", i+1))
		case line.Price.IsNegative():
			return refuse("line_price", fmt.Sprintf("line %d: price must not be negative", i+1))
		case !line.Price.Round(2).IsPositive():
			return refuse("line_price", fmt.Sprintf("line %d: price must be at least 0.01", i+1))
		}
	}
	return nil
}

// paymentCurrencyCode is the currency that every payment is recorded in.
const paymentCurrencyCode = "SAR"

// paidAmountError refuses a paid amount that is negative or has more than two decimal places.
func paidAmountError(paid decimal.Decimal) error {
	switch {
	case paid.IsNegative():
		return refuse("paid_amount_negative", "paid_amount must not be negative")
	case !paid.Round(2).Equal(paid):
		return refuse("paid_amount_precision", "paid_amount must have at most two decimal places")
	}
	return nil
}

// paymentAboveTotalError refuses a payment above the total of its bill.
func paymentAboveTotalError(paid, total decimal.Decimal) error {
	if paid.GreaterThan(total) {
		return refuse("paid_amount_above_total", "paid_amount is more than the bill total")
	}
	return nil
}

// recordSalePayment writes the payment of a sale in the sale's transaction. A draft's earlier payment is replaced, so a draft that is
// saved and then submitted is paid once. A payment above the bill's total is refused.
func recordSalePayment(c *gin.Context, qtx *db.Queries, request *model.AddBillRequest, billID uint64, draftID *uint64, userID int32) bool {
	ctx := c.Request.Context()
	if draftID != nil {
		if err := qtx.DeleteBillPayments(ctx, billID); err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return false
		}
	}
	if !request.PaidAmount.IsPositive() {
		return true
	}
	total, err := qtx.GetBillTotal(ctx, billID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return false
	}
	if err := paymentAboveTotalError(request.PaidAmount, total); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return false
	}
	currencyID, err := qtx.GetCurrencyIDByCode(ctx, paymentCurrencyCode)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("payment currency %s: %w", paymentCurrencyCode, err))
		return false
	}
	paidAt := effectiveDateOr(request.EffectiveDate)
	recordedBy := userID
	err = qtx.InsertBillPayment(ctx, db.InsertBillPaymentParams{
		BillID:        billID,
		Date:          paidAt,
		PaidAt:        paidAt,
		Amount:        request.PaidAmount,
		CurrencyID:    currencyID,
		PaymentMethod: request.PaymentMethod,
		RecordedBy:    &recordedBy,
	})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return false
	}
	return true
}

// openSaleBill returns the bill that a sale writes, and whether a draft is already final. A new sale gets a bill row with only the
// columns that have no value yet, and updateSaleBill writes the rest. A draft is locked before it is read, so two submits of one
// draft cannot both see it open.
func openSaleBill(c *gin.Context, qtx *db.Queries, request *model.AddBillRequest, draftID *uint64, userID int32) (uint64, bool, bool) {
	if draftID == nil {
		res, err := qtx.CreateBill(c.Request.Context(), db.CreateBillParams{
			EffectiveDate:   effectiveDateOr(nil),
			StoreID:         request.StoreId,
			MerchantID:      userID,
			Discount:        decimal.Zero,
			MaintenanceCost: decimal.Zero,
		})
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return 0, false, false
		}
		id, err := res.LastInsertId()
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return 0, false, false
		}
		return uint64(id), false, true
	}

	state, err := qtx.GetBillStateForUpdate(c.Request.Context(), *draftID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		c.AbortWithStatus(http.StatusNotFound)
		return 0, false, false
	case err != nil:
		c.AbortWithError(http.StatusInternalServerError, err)
		return 0, false, false
	}
	return *draftID, state > 0, true
}

// answerFinalDraft answers a submit of a draft that is already final. The same request id on this bill is a retry, and it gets the
// original result. Any other submit is refused as already final.
func answerFinalDraft(c *gin.Context, qtx *db.Queries, ident billIdentity, draftID *uint64) (uint64, bool) {
	original, found, ok := requestRecord(c, qtx, ident)
	if !ok {
		return 0, false
	}
	if found {
		return answerRecordedRequest(c, original, ident, draftID)
	}
	c.AbortWithStatusJSON(http.StatusConflict, gin.H{"message": "bill is already final"})
	return 0, false
}

// answerLostRace answers a sale whose request id a concurrent request recorded after the check. It reads the record with the given
// queries, which run outside the rolled-back transaction.
func answerLostRace(c *gin.Context, q *db.Queries, ident billIdentity, draftID *uint64) (uint64, bool) {
	original, found, ok := requestRecord(c, q, ident)
	if !ok {
		return 0, false
	}
	if found {
		return answerRecordedRequest(c, original, ident, draftID)
	}
	c.AbortWithStatusJSON(http.StatusConflict, gin.H{"message": duplicateSaleMessage})
	return 0, false
}

// requestRecord looks up the bill that recorded a sale's request id. A draft has no request id, so it has no record. A database error
// writes the error response.
func requestRecord(c *gin.Context, q *db.Queries, ident billIdentity) (db.GetBillByRequestIdentityRow, bool, bool) {
	if ident.PosID == "" {
		return db.GetBillByRequestIdentityRow{}, false, true
	}
	row, err := q.GetBillByRequestIdentity(c.Request.Context(), db.GetBillByRequestIdentityParams{
		PosID:         nilIfEmpty(ident.PosID),
		BillRequestID: nullableString(ident.BillRequestID),
	})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return row, false, true
	case err != nil:
		c.AbortWithError(http.StatusInternalServerError, err)
		return row, false, false
	}
	return row, true, true
}

// answerRecordedRequest answers a sale whose request id is already recorded on a bill. The same content for the same bill is a retry:
// it gets the original bill and writes nothing. Anything else is a conflict.
func answerRecordedRequest(c *gin.Context, original db.GetBillByRequestIdentityRow, ident billIdentity, draftID *uint64) (uint64, bool) {
	sameBill := draftID == nil || *draftID == original.ID
	sameContent := original.RequestFingerprint.Valid &&
		original.RequestFingerprint.String == ident.Fingerprint &&
		original.RequestFingerprintVersion == identity.FingerprintVersion
	if sameBill && sameContent {
		return original.ID, true
	}
	c.AbortWithStatusJSON(http.StatusConflict, gin.H{"code": requestConflictCode, "message": requestConflictMessage})
	return 0, false
}

// numberSale takes the next number of a final sale's branch series. A draft gets no number.
func numberSale(c *gin.Context, qtx *db.Queries, request *model.AddBillRequest) (uint64, *uint64, bool) {
	if request.State <= 0 {
		return 0, nil, true
	}
	number, err := allocateBillNumber(qtx, c, request.BranchID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return 0, nil, false
	}
	return number, &number, true
}

// updateSaleBill writes a sale's fields, number, identity and request fingerprint to its bill row. A new sale and a draft take this
// same statement, so the bill's columns are listed once. It returns the database error as it is, so a lost race can be told apart
// from a failure.
func updateSaleBill(c *gin.Context, qtx *db.Queries, request *model.AddBillRequest, billID uint64, sequence *uint64, ident billIdentity, userID int32) error {
	return qtx.UpdateBillByID(c.Request.Context(), db.UpdateBillByIDParams{
		ID:                        billID,
		EffectiveDate:             effectiveDateOr(request.EffectiveDate),
		PaymentDueDate:            request.PaymentDueDate,
		State:                     request.State,
		Discount:                  request.Discount,
		StoreID:                   request.StoreId,
		SequenceNumber:            sequence,
		MerchantID:                userID,
		MaintenanceCost:           request.MaintenanceCost,
		Note:                      request.Note,
		Username:                  request.UserName,
		ClientID:                  request.ClientID,
		UserPhoneNumber:           request.UserPhoneNumber,
		PaymentMethod:             request.PaymentMethod,
		BranchID:                  &request.BranchID,
		DeliverDate:               request.DeliverDate,
		PosID:                     nilIfEmpty(ident.PosID),
		BillRequestID:             nullableString(ident.BillRequestID),
		InvoiceUuid:               nullableString(ident.InvoiceUUID),
		RequestFingerprint:        nullableString(ident.Fingerprint),
		RequestFingerprintVersion: identity.FingerprintVersion,
		AmountPaid:                request.PaidAmount,
	})
}
