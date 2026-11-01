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

func (h *handler) AddBill(c *gin.Context) {
	request := model.AddBillRequest{
		State:         1,
		PaymentMethod: 0,
		PaidAmount:    decimal.NewFromInt(0),
	}
	if err := c.BindJSON(&request); err != nil {
		log.Printf("AddBill: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}

	ident, err := finalSaleIdentity(&request)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	userSession := GetSessionInfo(c)
	if !h.checkBillStoreAccess(c, request.StoreId) {
		return
	}

	tx, err := h.DB.Begin()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer tx.Rollback()
	qtx := h.queries.WithTx(tx)

	// A draft has no number, so its sequence_number stays NULL, and the unique key on (branch, number)
	// allows any number of drafts.
	var number uint64
	var sequence *uint64
	if request.State > 0 {
		number, err = allocateBillNumber(qtx, c, request.BranchID)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		sequence = &number
	}

	args := db.CreateBillParams{
		EffectiveDate:   effectiveDateOr(request.EffectiveDate),
		PaymentDueDate:  request.PaymentDueDate,
		State:           int32(request.State),
		Discount:        request.Discount,
		StoreID:         int32(request.StoreId),
		SequenceNumber:  sequence,
		MerchantID:      int32(userSession.id),
		MaintenanceCost: request.MaintenanceCost,
		Note:            request.Note,
		Username:        request.UserName,
		ClientID:        request.ClientID,
		UserPhoneNumber: request.UserPhoneNumber,
		PaymentMethod:   request.PaymentMethod,
		DeliverDate:     request.DeliverDate,
		BranchID:        &request.BranchID,
		PosID:           nilIfEmpty(ident.PosID),
		BillRequestID:   nullableString(ident.BillRequestID),
		InvoiceUuid:     nullableString(ident.InvoiceUUID),
	}
	res, err := qtx.CreateBill(c.Request.Context(), args)
	if err != nil {
		log.Printf("AddBill: %v", err)
		if IsDuplicate(err) {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{"message": duplicateSaleMessage})
			return
		}
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	id, err := res.LastInsertId()
	if err != nil {
		log.Printf("AddBill: %v", err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	sale := finalSale{billID: uint64(id), number: number, identity: ident}
	if !h.completeSale(c, qtx, tx, &request, sale, int32(userSession.id)) {
		return
	}
	if err := tx.Commit(); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusCreated, id)
}

func (h *handler) SubmitDraftBill(c *gin.Context) {
	billID, err := parseBillID(c.Param("id"))
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	request := model.AddBillRequest{
		State:         1,
		PaymentMethod: 0,
		PaidAmount:    decimal.NewFromInt(0),
	}
	if err := c.BindJSON(&request); err != nil {
		log.Printf("SubmitDraftBill: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}

	ident, err := finalSaleIdentity(&request)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	userSession := GetSessionInfo(c)
	if !h.checkBillStoreAccess(c, request.StoreId) {
		return
	}

	tx, err := h.DB.Begin()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer tx.Rollback()
	qtx := h.queries.WithTx(tx)

	// A bill is numbered once. Its state is read under a row lock, and a bill that is already final is refused.
	state, err := qtx.GetBillStateForUpdate(c.Request.Context(), billID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		c.AbortWithStatus(http.StatusNotFound)
		return
	case err != nil:
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	case state > 0:
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"message": "bill is already final"})
		return
	}

	var number uint64
	var sequence *uint64
	if request.State > 0 {
		number, err = allocateBillNumber(qtx, c, request.BranchID)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		sequence = &number
	}

	args := db.UpdateBillByIDParams{
		EffectiveDate:   effectiveDateOr(request.EffectiveDate),
		PaymentDueDate:  request.PaymentDueDate,
		State:           request.State,
		Discount:        request.Discount,
		StoreID:         request.StoreId,
		SequenceNumber:  sequence,
		MerchantID:      int32(userSession.id),
		MaintenanceCost: request.MaintenanceCost,
		Note:            request.Note,
		Username:        request.UserName,
		ClientID:        request.ClientID,
		UserPhoneNumber: request.UserPhoneNumber,
		ID:              billID,
		PaymentMethod:   request.PaymentMethod,
		DeliverDate:     request.DeliverDate,
		BranchID:        &request.BranchID,
		PosID:           nilIfEmpty(ident.PosID),
		BillRequestID:   nullableString(ident.BillRequestID),
		InvoiceUuid:     nullableString(ident.InvoiceUUID),
	}
	if err := qtx.UpdateBillByID(c.Request.Context(), args); err != nil {
		log.Printf("SubmitDraftBill: %v", err)
		if IsDuplicate(err) {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{"message": duplicateSaleMessage})
			return
		}
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if err := qtx.DeleteProductToBill(c.Request.Context(), billID); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	sale := finalSale{billID: billID, number: number, identity: ident}
	if !h.completeSale(c, qtx, tx, &request, sale, int32(userSession.id)) {
		return
	}
	if err := tx.Commit(); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.Status(http.StatusCreated)
}
