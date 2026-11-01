package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"
)

// AddBill saves a new sale or draft and answers with the new bill id.
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

// saveSale writes a sale in one transaction. A nil draftID creates a bill, and a draft id finalizes that draft. A final
// sale (state above 0) is numbered, identified, and queued for ZATCA in the same transaction. It returns the bill id.
// On failure it has already written the error response.
func (h *handler) saveSale(c *gin.Context, request *model.AddBillRequest, draftID *uint64) (uint64, bool) {
	ident, err := finalSaleIdentity(request)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return 0, false
	}
	userSession := GetSessionInfo(c)
	if !h.checkBillStoreAccess(c, request.StoreId) {
		return 0, false
	}

	tx, err := h.DB.Begin()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return 0, false
	}
	defer tx.Rollback()
	qtx := h.queries.WithTx(tx)
	userID := int32(userSession.id)

	billID, ok := openSaleBill(c, qtx, request, draftID, userID)
	if !ok {
		return 0, false
	}
	number, sequence, ok := numberSale(c, qtx, request)
	if !ok {
		return 0, false
	}
	if !updateSaleBill(c, qtx, request, billID, sequence, ident, userID) {
		return 0, false
	}
	if draftID != nil {
		if err := qtx.DeleteProductToBill(c.Request.Context(), billID); err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return 0, false
		}
	}
	sale := finalSale{billID: billID, number: number, identity: ident}
	if !h.completeSale(c, qtx, tx, request, sale, userID) {
		return 0, false
	}
	if err := tx.Commit(); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return 0, false
	}
	return billID, true
}

// openSaleBill returns the bill that a sale writes. A new sale gets a bill row with only the columns that have no value
// yet, and updateSaleBill writes the rest. A draft is locked and must still be open, because a final bill is never
// written again.
func openSaleBill(c *gin.Context, qtx *db.Queries, request *model.AddBillRequest, draftID *uint64, userID int32) (uint64, bool) {
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
			return 0, false
		}
		id, err := res.LastInsertId()
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return 0, false
		}
		return uint64(id), true
	}

	state, err := qtx.GetBillStateForUpdate(c.Request.Context(), *draftID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		c.AbortWithStatus(http.StatusNotFound)
		return 0, false
	case err != nil:
		c.AbortWithError(http.StatusInternalServerError, err)
		return 0, false
	case state > 0:
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"message": "bill is already final"})
		return 0, false
	}
	return *draftID, true
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

// updateSaleBill writes a sale's fields, number and identity to its bill row. A new sale and a draft take this same
// statement, so the bill's columns are listed in one place.
func updateSaleBill(c *gin.Context, qtx *db.Queries, request *model.AddBillRequest, billID uint64, sequence *uint64, ident billIdentity, userID int32) bool {
	err := qtx.UpdateBillByID(c.Request.Context(), db.UpdateBillByIDParams{
		ID:              billID,
		EffectiveDate:   effectiveDateOr(request.EffectiveDate),
		PaymentDueDate:  request.PaymentDueDate,
		State:           request.State,
		Discount:        request.Discount,
		StoreID:         request.StoreId,
		SequenceNumber:  sequence,
		MerchantID:      userID,
		MaintenanceCost: request.MaintenanceCost,
		Note:            request.Note,
		Username:        request.UserName,
		ClientID:        request.ClientID,
		UserPhoneNumber: request.UserPhoneNumber,
		PaymentMethod:   request.PaymentMethod,
		BranchID:        &request.BranchID,
		DeliverDate:     request.DeliverDate,
		PosID:           nilIfEmpty(ident.PosID),
		BillRequestID:   nullableString(ident.BillRequestID),
		InvoiceUuid:     nullableString(ident.InvoiceUUID),
	})
	if err == nil {
		return true
	}
	log.Printf("sale update: %v", err)
	if IsDuplicate(err) {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"message": duplicateSaleMessage})
	} else {
		c.AbortWithError(http.StatusInternalServerError, err)
	}
	return false
}
