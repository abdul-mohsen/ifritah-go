package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

const (
	atomicSaleFingerprintVersion uint16 = 1
	atomicSaleCurrencyID         int32  = 1
)

type atomicSaleIdentity struct {
	TenantID       uint64
	PosID          string
	BillRequestID  string
	Fingerprint    string
	FingerprintVer uint16
}

type atomicSaleResponse struct {
	ID            uint64 `json:"id"`
	InvoiceUUID   string `json:"invoice_uuid"`
	TenantID      uint64 `json:"tenant_id"`
	PosID         string `json:"pos_id"`
	BillRequestID string `json:"bill_request_id"`
	Replayed      bool   `json:"replayed"`
}

type atomicSaleOutboxPayload struct {
	DocType       string `json:"doc_type"`
	ID            uint64 `json:"id"`
	BranchID      uint32 `json:"branch_id"`
	DBName        string `json:"db_name"`
	TenantID      uint64 `json:"tenant_id"`
	PosID         string `json:"pos_id"`
	BillRequestID string `json:"bill_request_id"`
	EventID       string `json:"event_id"`
	EGSSequenceID string `json:"egs_sequence_id"`
	InvoiceUUID   string `json:"invoice_uuid"`
}

func (h *handler) addAtomicSale(c *gin.Context) {
	request := model.AddBillRequest{
		State:      1,
		PaidAmount: decimal.Zero,
		CurrencyID: atomicSaleCurrencyID,
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": invalidRequestDetail(err)})
		return
	}

	normalizeSaleRequest(&request)
	identity, err := canonicalAtomicSaleIdentity(c, &request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "invalid_sale_identity",
			"detail": err.Error(),
		})
		return
	}
	if request.PaidAmount.IsNegative() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "invalid_payment",
			"detail": "paid_amount must not be negative",
		})
		return
	}

	session := GetSessionInfo(c)
	ctx := c.Request.Context()
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer tx.Rollback()
	qtx := h.queries.WithTx(tx)

	existing, err := qtx.ResolveBillRequest(ctx, db.ResolveBillRequestParams{
		RequestFingerprint:        sql.NullString{String: identity.Fingerprint, Valid: true},
		RequestFingerprintVersion: identity.FingerprintVer,
		TenantID:                  uint64Ptr(identity.TenantID),
		PosID:                     stringPtr(identity.PosID),
		BillRequestID:             sql.NullString{String: identity.BillRequestID, Valid: true},
	})
	switch {
	case err == nil:
		h.respondAtomicSaleResolution(c, existing, identity)
		return
	case !errors.Is(err, sql.ErrNoRows):
		log.Printf("AddBill resolve request: %v", err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if _, err := qtx.GetSaleOwnership(ctx, db.GetSaleOwnershipParams{
		UserID:   int32(session.id),
		StoreID:  request.StoreId,
		BranchID: &request.BranchID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusForbidden, gin.H{
				"error":  "sale_ownership_rejected",
				"detail": "store, branch, and company ownership do not match",
			})
			return
		}
		log.Printf("AddBill ownership: %v", err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	products := atomicSaleProducts(&request)
	if err := validateAtomicSaleProducts(ctx, qtx, request.StoreId, products); err != nil {
		if errors.Is(err, errSaleProductOwnership) {
			c.JSON(http.StatusForbidden, gin.H{
				"error":  "sale_product_ownership_rejected",
				"detail": err.Error(),
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "invalid_sale_product",
			"detail": err.Error(),
		})
		return
	}

	sequenceNumber, err := allocateAtomicSaleSequence(ctx, qtx, request.BranchID, request.State)
	if err != nil {
		log.Printf("AddBill sequence: %v", err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	invoiceUUID, err := newAtomicSaleUUID()
	if err != nil {
		log.Printf("AddBill invoice identity: %v", err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	var eventID string

	tenantID := identity.TenantID
	posID := identity.PosID
	billRequestID := sql.NullString{String: identity.BillRequestID, Valid: true}
	fingerprint := sql.NullString{String: identity.Fingerprint, Valid: true}
	invoiceIdentity := sql.NullString{String: invoiceUUID, Valid: true}
	sequence := sequenceNumber

	res, err := qtx.CreateBill(ctx, db.CreateBillParams{
		EffectiveDate:             effectiveDateOr(request.EffectiveDate),
		PaymentDueDate:            request.PaymentDueDate,
		State:                     request.State,
		Discount:                  request.Discount,
		StoreID:                   request.StoreId,
		SequenceNumber:            &sequence,
		MerchantID:                int32(session.id),
		MaintenanceCost:           request.MaintenanceCost,
		Note:                      request.Note,
		Username:                  request.UserName,
		ClientID:                  request.ClientID,
		UserPhoneNumber:           request.UserPhoneNumber,
		PaymentMethod:             request.PaymentMethod,
		BranchID:                  &request.BranchID,
		DeliverDate:               request.DeliverDate,
		AmountPaid:                request.PaidAmount,
		TenantID:                  &tenantID,
		PosID:                     &posID,
		BillRequestID:             billRequestID,
		RequestFingerprint:        fingerprint,
		RequestFingerprintVersion: identity.FingerprintVer,
		InvoiceUuid:               invoiceIdentity,
	})
	if err != nil {
		if IsDuplicate(err) {
			_ = tx.Rollback()
			h.resolveAtomicSaleAfterDuplicate(c, identity)
			return
		}
		log.Printf("AddBill create bill: %v", err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	billID, err := res.LastInsertId()
	if err != nil || billID <= 0 {
		if err == nil {
			err = fmt.Errorf("invalid bill id %d", billID)
		}
		log.Printf("AddBill bill id: %v", err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if err := addProductToBill(qtx, c, products, uint64(billID)); err != nil {
		log.Printf("AddBill products: %v", err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if err := qtx.RefreshBillTotals(ctx, uint64(billID)); err != nil {
		log.Printf("AddBill refresh totals: %v", err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if request.PaidAmount.GreaterThan(decimal.Zero) {
		paymentDate := effectiveDateOr(request.PaymentDate)
		currencyID := request.CurrencyID
		if currencyID == 0 {
			currencyID = atomicSaleCurrencyID
		}
		recordedBy := int32(session.id)
		if _, err := qtx.InsertBillPayment(ctx, db.InsertBillPaymentParams{
			BillID:        uint64(billID),
			Date:          paymentDate,
			PaidAt:        paymentDate,
			Amount:        request.PaidAmount,
			CurrencyID:    currencyID,
			PaymentMethod: request.PaymentMethod,
			RecordedBy:    &recordedBy,
			Note:          request.Note,
		}); err != nil {
			log.Printf("AddBill payment: %v", err)
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
	}

	if request.State > 0 {
		enforcement, err := atomicSaleStockEnforcement(ctx, qtx)
		if err != nil {
			log.Printf("AddBill stock enforcement: %v", err)
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		if enforcement != model.StockEnforcementDisable {
			warnings, err := recordSaleMovements(
				qtx,
				c,
				uint64(billID),
				request.StoreId,
				request.Products,
				int32(sequenceNumber),
				enforcement,
				int32(session.id),
			)
			if err != nil {
				log.Printf("AddBill inventory: %s", sanitizeForLog(err.Error()))
				c.JSON(http.StatusBadRequest, gin.H{
					"detail": err.Error(),
					"type":   "stock_insufficient",
				})
				return
			}
			if len(warnings) > 0 {
				c.Header("X-Stock-Warning", "true")
			}
		}
	}

	if request.State > 0 {
		eventID, err = newAtomicSaleUUID()
		if err != nil {
			log.Printf("AddBill outbox identity: %v", err)
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		egsSequenceID := atomicSaleEGSSequenceID(request.BranchID, sequenceNumber)
		payload, payloadHash, err := atomicSaleOutboxPayloadBytes(atomicSaleOutboxPayload{
			DocType:       "bill",
			ID:            uint64(billID),
			BranchID:      request.BranchID,
			DBName:        env("DBNAME", ""),
			TenantID:      identity.TenantID,
			PosID:         identity.PosID,
			BillRequestID: identity.BillRequestID,
			EventID:       eventID,
			EGSSequenceID: egsSequenceID,
			InvoiceUUID:   invoiceUUID,
		})
		if err != nil {
			log.Printf("AddBill outbox payload: %v", err)
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		if err := insertAtomicSaleOutbox(ctx, qtx, db.InsertSaleOutboxParams{
			TenantID:      identity.TenantID,
			EventID:       eventID,
			AggregateType: "bill",
			AggregateID:   uint64(billID),
			PosID:         &posID,
			BillRequestID: billRequestID,
			InvoiceUuid:   invoiceIdentity,
			EgsSequenceID: stringPtr(egsSequenceID),
			Payload:       json.RawMessage(payload),
			PayloadFingerprint: sql.NullString{
				String: payloadHash,
				Valid:  true,
			},
			ContentHash: sql.NullString{
				String: payloadHash,
				Valid:  true,
			},
		}); err != nil {
			log.Printf("AddBill outbox: %v", err)
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("AddBill commit: %v", err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if h.outboxPublisher != nil && eventID != "" {
		if err := h.outboxPublisher.PublishOutbox(ctx, identity.TenantID, eventID); err != nil {
			log.Printf("AddBill outbox publish deferred: %v", err)
		}
	}
	c.JSON(http.StatusCreated, atomicSaleResponse{
		ID:            uint64(billID),
		InvoiceUUID:   invoiceUUID,
		TenantID:      identity.TenantID,
		PosID:         identity.PosID,
		BillRequestID: identity.BillRequestID,
		Replayed:      false,
	})
}

func (h *handler) resolveAtomicSaleAfterDuplicate(c *gin.Context, identity atomicSaleIdentity) {
	row, err := h.queries.ResolveBillRequest(c.Request.Context(), db.ResolveBillRequestParams{
		RequestFingerprint:        sql.NullString{String: identity.Fingerprint, Valid: true},
		RequestFingerprintVersion: identity.FingerprintVer,
		TenantID:                  uint64Ptr(identity.TenantID),
		PosID:                     stringPtr(identity.PosID),
		BillRequestID:             sql.NullString{String: identity.BillRequestID, Valid: true},
	})
	if err == nil {
		h.respondAtomicSaleResolution(c, row, identity)
		return
	}
	c.JSON(http.StatusConflict, gin.H{
		"error":  "sale_request_conflict",
		"detail": "bill request identity already exists",
	})
}

func (h *handler) respondAtomicSaleResolution(c *gin.Context, row db.ResolveBillRequestRow, identity atomicSaleIdentity) {
	invoiceUUID := ""
	if row.InvoiceUuid.Valid {
		invoiceUUID = row.InvoiceUuid.String
	}
	if row.RequestResolution == "replay" {
		c.JSON(http.StatusOK, atomicSaleResponse{
			ID:            row.ID,
			InvoiceUUID:   invoiceUUID,
			TenantID:      identity.TenantID,
			PosID:         identity.PosID,
			BillRequestID: identity.BillRequestID,
			Replayed:      true,
		})
		return
	}
	c.JSON(http.StatusConflict, gin.H{
		"error":           "sale_request_conflict",
		"detail":          "bill request id was already used with different content",
		"bill_id":         row.ID,
		"invoice_uuid":    invoiceUUID,
		"tenant_id":       identity.TenantID,
		"pos_id":          identity.PosID,
		"bill_request_id": identity.BillRequestID,
	})
}

func normalizeSaleRequest(request *model.AddBillRequest) {
	for i := range request.ManualProducts {
		request.ManualProducts[i].ProductId = nil
	}
	if request.Products == nil {
		request.Products = []model.BillProduct{}
	}
	if request.ManualProducts == nil {
		request.ManualProducts = []model.BillProduct{}
	}
}

func canonicalAtomicSaleIdentity(c *gin.Context, request *model.AddBillRequest) (atomicSaleIdentity, error) {
	tenantID := request.TenantID
	if tenantID == 0 {
		raw := strings.TrimSpace(c.GetHeader("X-Tenant-ID"))
		if raw == "" {
			return atomicSaleIdentity{}, fmt.Errorf("tenant_id is required")
		}
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || parsed == 0 {
			return atomicSaleIdentity{}, fmt.Errorf("tenant_id must be a positive integer")
		}
		tenantID = parsed
	}
	posID := strings.TrimSpace(request.PosID)
	if posID == "" {
		posID = strings.TrimSpace(c.GetHeader("X-POS-ID"))
	}
	if posID == "" {
		return atomicSaleIdentity{}, fmt.Errorf("pos_id is required")
	}
	if len(posID) > 64 {
		return atomicSaleIdentity{}, fmt.Errorf("pos_id must be at most 64 characters")
	}
	billRequestID := strings.TrimSpace(request.BillRequestID)
	if billRequestID == "" {
		billRequestID = strings.TrimSpace(c.GetHeader("X-Bill-Request-ID"))
	}
	if billRequestID == "" {
		billRequestID = strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	}
	if billRequestID == "" {
		billRequestID = strings.TrimSpace(c.GetHeader("X-Request-ID"))
	}
	if billRequestID == "" {
		return atomicSaleIdentity{}, fmt.Errorf("bill_request_id is required")
	}
	if len(billRequestID) > 36 {
		return atomicSaleIdentity{}, fmt.Errorf("bill_request_id must be at most 36 characters")
	}

	request.TenantID = tenantID
	request.PosID = posID
	request.BillRequestID = billRequestID
	fingerprint, err := atomicSaleFingerprint(*request)
	if err != nil {
		return atomicSaleIdentity{}, fmt.Errorf("fingerprint request: %w", err)
	}
	return atomicSaleIdentity{
		TenantID:       tenantID,
		PosID:          posID,
		BillRequestID:  billRequestID,
		Fingerprint:    fingerprint,
		FingerprintVer: atomicSaleFingerprintVersion,
	}, nil
}

func atomicSaleFingerprint(request model.AddBillRequest) (string, error) {
	request.TenantID = 0
	request.PosID = ""
	request.BillRequestID = ""
	data, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

var errSaleProductOwnership = errors.New("product does not belong to the requested store")

func validateAtomicSaleProducts(ctx context.Context, qtx *db.Queries, storeID int32, products []model.BillProduct) error {
	seen := make(map[uint64]struct{}, len(products))
	for _, product := range products {
		if product.ProductId == nil {
			if product.Quantity.LessThanOrEqual(decimal.Zero) || product.Price.LessThanOrEqual(decimal.Zero) {
				return fmt.Errorf("manual product price and quantity must be positive")
			}
			continue
		}
		productID := *product.ProductId
		if productID == 0 {
			return fmt.Errorf("product id must be positive")
		}
		if product.Quantity.LessThanOrEqual(decimal.Zero) || product.Price.LessThanOrEqual(decimal.Zero) {
			return fmt.Errorf("product %d price and quantity must be positive", productID)
		}
		if _, ok := seen[productID]; ok {
			continue
		}
		seen[productID] = struct{}{}
		if _, err := qtx.GetSaleProductForUpdate(ctx, db.GetSaleProductForUpdateParams{
			ProductID: productID,
			StoreID:   storeID,
		}); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w: %d", errSaleProductOwnership, productID)
			}
			return err
		}
	}
	return nil
}

func atomicSaleProducts(request *model.AddBillRequest) []model.BillProduct {
	products := make([]model.BillProduct, 0, len(request.Products)+len(request.ManualProducts)+1)
	products = append(products, request.Products...)
	products = append(products, request.ManualProducts...)
	if request.MaintenanceCost.GreaterThan(decimal.Zero) {
		products = append(products, model.BillProduct{
			Name:     model.MaintenanceCost,
			Price:    request.MaintenanceCost,
			Quantity: decimal.NewFromInt(1),
		})
	}
	return products
}

func allocateAtomicSaleSequence(ctx context.Context, qtx *db.Queries, branchID uint32, state int32) (uint64, error) {
	if state <= 0 {
		return 0, nil
	}
	res, err := qtx.AllocateBranchSequence(ctx, db.AllocateBranchSequenceParams{
		BranchID: branchID,
		Scope:    "bill",
	})
	if err != nil {
		return 0, err
	}
	sequence, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if sequence <= 0 {
		return 0, fmt.Errorf("sequence allocator returned %d", sequence)
	}
	return uint64(sequence), nil
}

func atomicSaleStockEnforcement(ctx context.Context, qtx *db.Queries) (string, error) {
	enforcement, err := qtx.GetStockEnforcement(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return model.StockEnforcementDisable, nil
	}
	if err != nil {
		return "", err
	}
	return enforcement, nil
}

func newAtomicSaleUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf(
		"%x-%x-%x-%x-%x",
		value[0:4],
		value[4:6],
		value[6:8],
		value[8:10],
		value[10:16],
	), nil
}

func atomicSaleEGSSequenceID(branchID uint32, sequenceNumber uint64) string {
	return fmt.Sprintf("%d:%d", branchID, sequenceNumber)
}

func uint64Ptr(value uint64) *uint64 {
	return &value
}

func stringPtr(value string) *string {
	return &value
}
