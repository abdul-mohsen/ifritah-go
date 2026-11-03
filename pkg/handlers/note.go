package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/identity"
	"ifritah/web-service-gin/pkg/model"
	"ifritah/web-service-gin/pkg/outbox"
)

// noteStateFinal is the state of a note that is written. A note is issued when it is written, so it is never a draft.
const noteStateFinal int32 = 1

// The limits of a note. They match the columns the note is stored in.
const (
	maxNoteReasonLen = 512
	maxNoteLineName  = 255
	maxNoteVATRate   = 100
)

// noteDuplicateMessage answers a note whose request a concurrent request recorded first and that cannot be read again.
const noteDuplicateMessage = "this note request was already recorded"

// noteConflictMessage answers a note whose request id is recorded for another note or bill. The code is requestConflictCode.
const noteConflictMessage = "this request id was already used for a different note or bill"

// noteBillNotFound answers a note whose bill does not exist or is deleted.
const noteBillNotFound = "bill not found"

// noteRecord is the note that recorded a request id, as a retry reads it.
type noteRecord struct {
	id          uint64
	billID      uint64
	fingerprint sql.NullString
	version     uint16
}

// noteLine is one line of a credit or debit note, as it is written. A credit line also names the bill line it credits, and the
// catalog product that goes back into stock.
type noteLine struct {
	billProductID int32
	productID     *uint64
	name          *string
	quantity      decimal.Decimal
	price         decimal.Decimal
	discount      decimal.Decimal
	vat           decimal.Decimal
	total         decimal.Decimal
}

// creditFingerprint is the fingerprint of a credit note request, which is the request without its identity. A retry of the same note
// has the same fingerprint, and a different note under the same identity does not.
func creditFingerprint(request model.CreditNoteRequest) (string, error) {
	request.PosID, request.BillRequestID = "", ""
	return identity.Fingerprint(request)
}

// debitFingerprint is creditFingerprint for a debit note.
func debitFingerprint(request model.DebitNoteRequest) (string, error) {
	request.PosID, request.BillRequestID = "", ""
	return identity.Fingerprint(request)
}

// noteIdentity checks the request identity of a note and creates the note's invoice UUID. The UUID is created once, here, and a
// retry gets the original note with its own UUID.
func noteIdentity(posID, requestID, fingerprint string) (billIdentity, error) {
	if err := identity.ValidatePosID(posID); err != nil {
		return billIdentity{}, err
	}
	if err := identity.ValidateBillRequestID(requestID); err != nil {
		return billIdentity{}, err
	}
	invoiceUUID, err := identity.NewUUID()
	if err != nil {
		return billIdentity{}, err
	}
	return billIdentity{PosID: posID, BillRequestID: requestID, InvoiceUUID: invoiceUUID, Fingerprint: fingerprint}, nil
}

// normalizeNoteReason trims the reason of a note and checks it. The reason is the instruction note that the ZATCA document carries.
func normalizeNoteReason(reason *string) error {
	*reason = strings.TrimSpace(*reason)
	switch {
	case *reason == "":
		return errors.New("reason is required")
	case utf8.RuneCountInString(*reason) > maxNoteReasonLen:
		return fmt.Errorf("reason must be at most %d characters", maxNoteReasonLen)
	}
	return nil
}

// verifyNoteAmount checks the amount of a note: above zero, with at most two decimal places.
func verifyNoteAmount(amount decimal.Decimal) error {
	switch {
	case !amount.IsPositive():
		return errors.New("amount must be above zero")
	case !hasDecimalPlaces(amount, 2):
		return errors.New("amount must have at most two decimal places")
	}
	return nil
}

// verifyLineAmounts checks the quantity, price and VAT rate of a note line, with the precision that the line stores.
func verifyLineAmounts(quantity, price, vat decimal.Decimal) error {
	switch {
	case !quantity.IsPositive() || !hasDecimalPlaces(quantity, 3):
		return errors.New("quantity must be above zero, with at most three decimal places")
	case !price.IsPositive() || !hasDecimalPlaces(price, 2):
		return errors.New("price must be above zero, with at most two decimal places")
	case vat.IsNegative() || vat.GreaterThan(decimal.NewFromInt(maxNoteVATRate)) || !hasDecimalPlaces(vat, 2):
		return errors.New("VAT rate must be from 0 to 100, with at most two decimal places")
	}
	return nil
}

// hasDecimalPlaces reports whether d has no more than places digits after the decimal point.
func hasDecimalPlaces(d decimal.Decimal, places int32) bool {
	return d.Round(places).Equal(d)
}

// verifyCreditRequest checks what a credit note can be checked from its own request: its reason, its amount, and the shape of its
// lines. The bill and its lines are checked when the note is written. It trims the reason, so the fingerprint covers the trimmed text.
func verifyCreditRequest(request *model.CreditNoteRequest) error {
	if err := normalizeNoteReason(&request.Reason); err != nil {
		return err
	}
	if err := verifyNoteAmount(request.Amount); err != nil {
		return err
	}
	if len(request.Lines) == 0 {
		return errors.New("a credit note needs at least one line")
	}
	for i, line := range request.Lines {
		if line.BillProductID == 0 {
			return fmt.Errorf("line %d: bill_product_id is required", i+1)
		}
		if err := verifyLineAmounts(line.Quantity, line.Price, line.VAT); err != nil {
			return fmt.Errorf("line %d: %w", i+1, err)
		}
	}
	return nil
}

// verifyDebitRequest checks what a debit note can be checked from its own request. It trims the reason and the line names, so the
// fingerprint covers the trimmed text.
func verifyDebitRequest(request *model.DebitNoteRequest) error {
	if err := normalizeNoteReason(&request.Reason); err != nil {
		return err
	}
	if err := verifyNoteAmount(request.Amount); err != nil {
		return err
	}
	if len(request.Lines) == 0 {
		return errors.New("a debit note needs at least one line")
	}
	for i := range request.Lines {
		line := &request.Lines[i]
		line.Name = strings.TrimSpace(line.Name)
		switch {
		case line.Name == "":
			return fmt.Errorf("line %d: name is required", i+1)
		case utf8.RuneCountInString(line.Name) > maxNoteLineName:
			return fmt.Errorf("line %d: name must be at most %d characters", i+1, maxNoteLineName)
		}
		if err := verifyLineAmounts(line.Quantity, line.Price, line.VAT); err != nil {
			return fmt.Errorf("line %d: %w", i+1, err)
		}
	}
	return nil
}

// noteLineTotal returns the total of a note line with VAT. It rounds the way the database rounds the generated columns of bill_product
// and of the note lines, so the total that the backend checks is the total that the database stores.
func noteLineTotal(quantity, price, discount, vat decimal.Decimal) decimal.Decimal {
	beforeVAT := price.Mul(quantity).Round(2).Sub(discount).Round(2)
	vatAmount := beforeVAT.Mul(vat).Shift(-2).Round(2)
	return beforeVAT.Add(vatAmount).Round(2)
}

// creditLines turns the lines of a credit note into the lines that it writes. Each line credits one line of the bill. Its price and VAT
// rate must be the bill line's, and its quantity must not be more than the bill line has left after the earlier notes. The discount of
// the bill line is shared in proportion to the quantity, so the lines of all the notes add up to the discount of the bill line.
func creditLines(requested []model.CreditNoteLine, billLines map[uint64]db.ListBillLinesForNoteRow, credited map[uint64]decimal.Decimal) ([]noteLine, error) {
	pending := make(map[uint64]decimal.Decimal, len(requested))
	lines := make([]noteLine, 0, len(requested))
	for i, requestedLine := range requested {
		n := i + 1
		billLine, ok := billLines[requestedLine.BillProductID]
		if !ok {
			return nil, fmt.Errorf("line %d: bill_product_id %d is not a line of this bill", n, requestedLine.BillProductID)
		}
		if billLine.Vat == nil {
			return nil, fmt.Errorf("line %d: the bill line has no VAT rate", n)
		}
		if !requestedLine.Price.Equal(billLine.Price) {
			return nil, fmt.Errorf("line %d: price must be the bill line's price, %s", n, billLine.Price.StringFixed(2))
		}
		if !requestedLine.VAT.Equal(*billLine.Vat) {
			return nil, fmt.Errorf("line %d: VAT rate must be the bill line's rate, %s", n, billLine.Vat.StringFixed(2))
		}
		pending[requestedLine.BillProductID] = pending[requestedLine.BillProductID].Add(requestedLine.Quantity)
		open := billLine.Quantity.Sub(credited[requestedLine.BillProductID])
		if pending[requestedLine.BillProductID].GreaterThan(open) {
			return nil, fmt.Errorf("line %d: quantity is more than the %s the bill line has left", n, open.String())
		}
		discount := billLine.Discount.Mul(requestedLine.Quantity).Div(billLine.Quantity).Round(2)
		lines = append(lines, noteLine{
			billProductID: billLine.ID,
			productID:     billLine.ProductID,
			name:          billLine.Name,
			quantity:      requestedLine.Quantity,
			price:         billLine.Price,
			discount:      discount,
			vat:           *billLine.Vat,
			total:         noteLineTotal(requestedLine.Quantity, billLine.Price, discount, *billLine.Vat),
		})
	}
	return lines, nil
}

// debitLines turns the lines of a debit note into the lines that it writes. A debit line is a new charge, so it has no bill line.
func debitLines(requested []model.DebitNoteLine) []noteLine {
	lines := make([]noteLine, 0, len(requested))
	for _, line := range requested {
		name := line.Name
		lines = append(lines, noteLine{
			name:     &name,
			quantity: line.Quantity,
			price:    line.Price,
			discount: decimal.Zero,
			vat:      line.VAT,
			total:    noteLineTotal(line.Quantity, line.Price, decimal.Zero, line.VAT),
		})
	}
	return lines
}

// checkNoteTotal returns the total of the lines, and refuses an amount that is not that total.
func checkNoteTotal(amount decimal.Decimal, lines []noteLine) (decimal.Decimal, error) {
	total := decimal.Zero
	for _, line := range lines {
		total = total.Add(line.total)
	}
	if !amount.Equal(total) {
		return total, fmt.Errorf("amount must be the total of the lines, %s", total.StringFixed(2))
	}
	return total, nil
}

// creditRemaining returns what a bill can still credit: its total less the amounts of the notes already recorded for it.
func creditRemaining(total decimal.Decimal, recorded []decimal.Decimal) decimal.Decimal {
	remaining := total
	for _, amount := range recorded {
		remaining = remaining.Sub(amount)
	}
	return remaining
}

// checkNoteStoreAccess reads the store of the bill that a note refers to, and checks that the caller may use that store. It runs before
// the transaction, like the check of a sale. It writes the error response and returns false when the caller may not continue.
func (h *handler) checkNoteStoreAccess(c *gin.Context, billID uint64) bool {
	storeID, err := h.queries.GetBillStoreID(c.Request.Context(), billID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": noteBillNotFound})
		return false
	case err != nil:
		c.AbortWithError(http.StatusInternalServerError, err)
		return false
	}
	return h.checkBillStoreAccess(c, storeID)
}

// lockNoteBill locks the bill that a note refers to, until the transaction ends, so two notes on one bill are written one after the
// other. A note needs a final bill of the branch that the request names. It writes the error response and returns false when the bill
// cannot take a note.
func lockNoteBill(c *gin.Context, qtx *db.Queries, billID uint64, branchID uint32) (db.LockBillForNoteRow, bool) {
	bill, err := qtx.LockBillForNote(c.Request.Context(), billID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": noteBillNotFound})
		return bill, false
	case err != nil:
		c.AbortWithError(http.StatusInternalServerError, err)
		return bill, false
	}
	if bill.BranchID == nil || *bill.BranchID != branchID {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "the bill belongs to another branch"})
		return bill, false
	}
	switch {
	case bill.State < 0:
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": noteBillNotFound})
		return bill, false
	case bill.State == 0:
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"message": "a draft cannot be credited or debited; only a final bill can"})
		return bill, false
	}
	return bill, true
}

// creditRecordOf reads the credit note that recorded a request id. It reports false when no note recorded it.
func creditRecordOf(c *gin.Context, q *db.Queries, ident billIdentity) (noteRecord, bool, error) {
	row, err := q.GetCreditNoteByRequestIdentity(c.Request.Context(), db.GetCreditNoteByRequestIdentityParams{
		PosID:         nilIfEmpty(ident.PosID),
		BillRequestID: nullableString(ident.BillRequestID),
	})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return noteRecord{}, false, nil
	case err != nil:
		return noteRecord{}, false, err
	}
	return noteRecord{id: row.ID, billID: row.BillID, fingerprint: row.RequestFingerprint, version: row.RequestFingerprintVersion}, true, nil
}

// debitRecordOf reads the debit note that recorded a request id. It reports false when no note recorded it.
func debitRecordOf(c *gin.Context, q *db.Queries, ident billIdentity) (noteRecord, bool, error) {
	row, err := q.GetDebitNoteByRequestIdentity(c.Request.Context(), db.GetDebitNoteByRequestIdentityParams{
		PosID:         nilIfEmpty(ident.PosID),
		BillRequestID: nullableString(ident.BillRequestID),
	})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return noteRecord{}, false, nil
	case err != nil:
		return noteRecord{}, false, err
	}
	return noteRecord{id: row.ID, billID: row.BillID, fingerprint: row.RequestFingerprint, version: row.RequestFingerprintVersion}, true, nil
}

// billRequestTaken reports whether a bill already has the request id.
func billRequestTaken(c *gin.Context, qtx *db.Queries, ident billIdentity) (bool, error) {
	_, err := qtx.GetBillByRequestIdentity(c.Request.Context(), db.GetBillByRequestIdentityParams{
		PosID:         nilIfEmpty(ident.PosID),
		BillRequestID: nullableString(ident.BillRequestID),
	})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	case err != nil:
		return false, err
	}
	return true, nil
}

// otherNoteRequestTaken reports whether a request id is already used by a bill, or by a note of the other kind. The daemon claims each
// request id once for the whole tenant database, for bills and for both kinds of note. A note that shares its id with another document
// would be refused by the daemon and never invoiced, so the backend refuses it first. A note of the same kind is checked by its writer.
func otherNoteRequestTaken(c *gin.Context, qtx *db.Queries, ident billIdentity, docType string) (bool, error) {
	taken, err := billRequestTaken(c, qtx, ident)
	if err != nil || taken {
		return taken, err
	}
	var found bool
	if docType == identity.DocCredit {
		_, found, err = debitRecordOf(c, qtx, ident)
	} else {
		_, found, err = creditRecordOf(c, qtx, ident)
	}
	return found, err
}

// answerRecordedNote answers a note whose request id is already recorded. The same content for the same bill is a retry: it gets the
// original note's id and writes nothing. Anything else is a conflict.
func answerRecordedNote(c *gin.Context, rec noteRecord, billID uint64, ident billIdentity) (uint64, bool) {
	sameContent := rec.fingerprint.Valid && rec.fingerprint.String == ident.Fingerprint && rec.version == identity.FingerprintVersion
	if rec.billID == billID && sameContent {
		return rec.id, true
	}
	c.AbortWithStatusJSON(http.StatusConflict, gin.H{"code": requestConflictCode, "message": noteConflictMessage})
	return 0, false
}

// answerLostNoteRace answers a note whose request id a concurrent request recorded after the check. The record is read again outside the
// rolled-back transaction, so the winner is visible.
func answerLostNoteRace(c *gin.Context, rec noteRecord, found bool, err error, billID uint64, ident billIdentity) (uint64, bool) {
	switch {
	case err != nil:
		c.AbortWithError(http.StatusInternalServerError, err)
		return 0, false
	case found:
		return answerRecordedNote(c, rec, billID, ident)
	}
	c.AbortWithStatusJSON(http.StatusConflict, gin.H{"message": noteDuplicateMessage})
	return 0, false
}

// queueNoteEvent writes the ZATCA event of a note into the note's transaction. The drainer publishes it after the commit.
func (h *handler) queueNoteEvent(c *gin.Context, tx *sql.Tx, docType string, noteID uint64, branchID uint32, ident billIdentity) error {
	event, err := h.pub.DocumentEvent(docType, noteID, branchID, ident)
	if err != nil {
		return err
	}
	return outbox.Append(c.Request.Context(), tx, event)
}

// restoreCreditStock puts the catalog products of a credit note back into stock, in the enforcement mode of the settings, as a sale takes
// them out. When stock tracking is disabled, nothing moves. A line without a catalog product has no stock.
func (h *handler) restoreCreditStock(c *gin.Context, qtx *db.Queries, lines []noteLine, storeID int32, noteID uint64, userID int32) error {
	var catalog []noteLine
	for _, line := range lines {
		if line.productID != nil {
			catalog = append(catalog, line)
		}
	}
	if len(catalog) == 0 || h.getStockEnforcementMode(c) == model.StockEnforcementDisable {
		return nil
	}
	now := time.Now()
	note := fmt.Sprintf("إشعار دائن #%d", noteID)
	uid := userID
	for _, line := range catalog {
		productID := *line.productID
		if err := updateProductQuantity(qtx, c, productID, line.quantity); err != nil {
			return fmt.Errorf("restore stock of product %d: %w", productID, err)
		}
		if err := insertStockMovement(qtx, c, productID, storeID, line.quantity,
			model.MovementTypeCreditNote, model.ReferenceTypeCreditNote, &noteID, nil, nil, &note, &uid, now); err != nil {
			return fmt.Errorf("record credit movement for product %d: %w", productID, err)
		}
	}
	return nil
}
