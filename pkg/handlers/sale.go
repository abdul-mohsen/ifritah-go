package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/identity"
	"ifritah/web-service-gin/pkg/model"
	"ifritah/web-service-gin/pkg/outbox"
)

// billSequenceScope names the bill series in branch_sequence.
const billSequenceScope = "bill"

// duplicateSaleMessage answers a sale whose pos_id and bill_request_id were already recorded.
const duplicateSaleMessage = "this sale request was already recorded"

// billIdentity is the request identity and the invoice UUID of a final sale. A draft has neither.
type billIdentity struct {
	PosID         string
	BillRequestID string
	InvoiceUUID   string
}

// finalSale is a sale as it is written: the bill, its number in the branch series, and its identity.
type finalSale struct {
	billID   uint64
	number   uint64
	identity billIdentity
}

// finalSaleIdentity returns the identity of a sale. A final sale (state above 0) needs a valid pos_id and
// bill_request_id, and it gets its invoice UUID here, once. A draft has no identity and no UUID.
func finalSaleIdentity(request *model.AddBillRequest) (billIdentity, error) {
	if request.State <= 0 {
		return billIdentity{}, nil
	}
	if err := identity.ValidatePosID(request.PosID); err != nil {
		return billIdentity{}, err
	}
	if err := identity.ValidateBillRequestID(request.BillRequestID); err != nil {
		return billIdentity{}, err
	}
	invoiceUUID, err := identity.NewUUID()
	if err != nil {
		return billIdentity{}, err
	}
	return billIdentity{PosID: request.PosID, BillRequestID: request.BillRequestID, InvoiceUUID: invoiceUUID}, nil
}

// allocateBillNumber takes the next number of the branch's bill series. The increment locks the branch's
// counter in branch_sequence until the sale commits, so two sales of one branch cannot take the same
// number. An error is returned as it is. There is no fallback number.
func allocateBillNumber(qtx *db.Queries, c *gin.Context, branchID uint32) (uint64, error) {
	res, err := qtx.AllocateBranchSequence(c.Request.Context(), db.AllocateBranchSequenceParams{
		BranchID: branchID,
		Scope:    billSequenceScope,
	})
	if err != nil {
		return 0, fmt.Errorf("allocate bill number: %w", err)
	}
	number, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("bill number: %w", err)
	}
	if number < 1 {
		return 0, fmt.Errorf("bill number: branch %d returned %d", branchID, number)
	}
	return uint64(number), nil
}

// nilIfEmpty returns s, or NULL when s is empty. Drafts keep their identity columns NULL.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// nullableString is optionalString for the columns that sqlc maps to sql.NullString.
func nullableString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// saleProducts returns the lines of a sale: the catalog products, then the manual products, then the
// maintenance cost as a line of its own when it is positive.
func saleProducts(request *model.AddBillRequest) []model.BillProduct {
	// A manual line has no catalog product, so a product id that the frontend sends with it is dropped.
	for i := range request.ManualProducts {
		request.ManualProducts[i].ProductId = nil
	}

	products := append(request.Products, request.ManualProducts...)

	if request.MaintenanceCost.GreaterThan(decimal.Zero) {
		product := model.BillProduct{
			Name:     model.MaintenanceCost,
			Price:    request.MaintenanceCost,
			Quantity: decimal.NewFromInt(1),
		}
		products = append(products, product)
	}
	return products
}

// recordSaleStock deducts the catalog products of a final sale from stock, in the enforcement mode of the
// settings. It reports whether a product was short in warn mode. In enforce mode a short product is an error.
func (h *handler) recordSaleStock(c *gin.Context, qtx *db.Queries, request *model.AddBillRequest, billID, number uint64, userID int32) (bool, error) {
	enforcement := h.getStockEnforcementMode(c)
	if enforcement == model.StockEnforcementDisable || request.State <= 0 {
		return false, nil
	}
	warnings, err := recordSaleMovements(
		qtx, c, billID, request.StoreId,
		request.Products, int32(number),
		enforcement, userID,
	)
	return len(warnings) > 0, err
}

// queueSaleEvent writes the ZATCA event of a final sale into the sale's transaction. The drainer publishes
// it after the commit. A draft has no identity, so it has no event.
func (h *handler) queueSaleEvent(c *gin.Context, tx *sql.Tx, sale finalSale, branchID uint32) error {
	if sale.identity.InvoiceUUID == "" {
		return nil
	}
	event, err := h.pub.BillEvent(sale.billID, branchID, sale.identity)
	if err != nil {
		return err
	}
	return outbox.Append(c.Request.Context(), tx, event)
}

// completeSale writes the lines, totals, stock movements and ZATCA event of a sale, inside the sale's
// transaction. It reports false after it has written the error response.
func (h *handler) completeSale(c *gin.Context, qtx *db.Queries, tx *sql.Tx, request *model.AddBillRequest, sale finalSale, userID int32) bool {
	if err := addProductToBill(qtx, c, saleProducts(request), sale.billID); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return false
	}
	if err := qtx.RefreshBillTotals(c.Request.Context(), sale.billID); err != nil {
		log.Printf("sale refresh totals: %v", err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return false
	}
	warned, err := h.recordSaleStock(c, qtx, request, sale.billID, sale.number, userID)
	if err != nil {
		log.Printf("sale stock: %s", sanitizeForLog(err.Error()))
		// enforce mode: block if insufficient stock
		c.JSON(http.StatusBadRequest, gin.H{
			"detail": err.Error(),
			"type":   "stock_insufficient",
		})
		return false
	}
	if warned {
		// warn mode: set header with stock warnings
		c.Header("X-Stock-Warning", "true")
	}
	if err := h.queueSaleEvent(c, tx, sale, request.BranchID); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return false
	}
	return true
}
