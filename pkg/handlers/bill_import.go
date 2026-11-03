package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/shopspring/decimal"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/identity"
	"ifritah/web-service-gin/pkg/model"
)

// maxImportRows is the most rows that one check takes. A larger batch is refused before any row is read.
const maxImportRows = 500

// The codes of the issues that a check reports. A duplicate is a warning, and every other code is an error that blocks its row.
// The codes of the sale path's refusals are the codes of saleRefusal.
const (
	importInvalidRequest      = "invalid_request"
	importNotFinal            = "not_final"
	importStoreNotAllowed     = "store_not_allowed"
	importStoreNotInBranch    = "store_not_in_branch"
	importBranchNotFound      = "branch_not_found"
	importBranchNotRegistered = "branch_not_registered"
	importSellerVATMissing    = "seller_vat_missing"
	importEGSIDMissing        = "egs_id_missing"
	importDateInvalid         = "date_invalid"
	importDateInFuture        = "date_in_future"
	importDateOutsideWindow   = "date_outside_reporting_window"
	importStockShort          = "stock_insufficient"
	importDuplicateSale       = "duplicate_sale"
	importDuplicateInBatch    = "duplicate_in_batch"
)

// defaultLineVATRate is the VAT rate that bill_product gives a line which the sale path writes: the column default, 15 percent.
var defaultLineVATRate = decimal.RequireFromString("0.15")

// importRows is the body of a check: the rows, each one the JSON of a final sale.
type importRows struct {
	Rows []json.RawMessage `json:"rows" binding:"required"`
}

// importIssue is one error or warning of a row.
type importIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// importRowResult is the answer for one row. Index is the position of the row in the request. RequestFingerprint is the fingerprint of the
// sale, which the sale path stores for duplicate detection. It is null for a row that is not a sale request.
type importRowResult struct {
	Index              int           `json:"index"`
	Valid              bool          `json:"valid"`
	Errors             []importIssue `json:"errors"`
	Warnings           []importIssue `json:"warnings"`
	DuplicateOf        *uint64       `json:"duplicate_of"`
	RequestFingerprint *string       `json:"request_fingerprint"`
}

// importBranch is what the daemon reads for a branch: the VAT of the branch's company, the EGS id in the branch's ZATCA config, and
// whether the daemon runs the branch (GetDaemonBranch). found is false when there is no such branch.
type importBranch struct {
	found      bool
	vat        string
	egsID      string
	daemonRuns bool
}

// importChecker is what the rows of one check share. The stores are the caller's, and the stock mode is the setting. Each branch is
// read once. seen holds the first row of each content, and taken holds the stock that the valid rows so far would use.
type importChecker struct {
	h        *handler
	storeIDs []int32
	stock    string
	now      time.Time
	branches map[uint32]importBranch
	seen     map[string]int
	taken    map[uint64]decimal.Decimal
}

// CheckBillImport checks the rows of a bulk import of sales, and answers each row in order. A row is checked as a final sale: with
// the rules of the sale path, the values as the database stores them, the date rules of the ZATCA daemon, the seller VAT, the EGS id,
// and the daemon's rule for running the branch, and against the stock that the valid rows before it would take. A row whose content is
// already a bill on its branch is a warning, and so are a row that repeats an earlier row of the same check and a sale older than the
// reporting window. The check writes nothing: no bill, number, payment, stock movement or outbox event. It ignores pos_id and
// bill_request_id, which the sale path uses for its replay and conflict rules and which are not part of the content.
func (h *handler) CheckBillImport(c *gin.Context) {
	var body importRows
	if err := c.BindJSON(&body); err != nil {
		log.Printf("CheckBillImport: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}
	if len(body.Rows) > maxImportRows {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("an import check takes at most %d rows", maxImportRows)})
		return
	}
	checker := importChecker{
		h:        h,
		storeIDs: h.getStoreIds(c),
		stock:    h.getStockEnforcementMode(c),
		now:      time.Now(),
		branches: map[uint32]importBranch{},
		seen:     map[string]int{},
		taken:    map[uint64]decimal.Decimal{},
	}
	rows := make([]importRowResult, 0, len(body.Rows))
	for i, raw := range body.Rows {
		row, err := checker.checkRow(c, i, raw)
		if err != nil {
			log.Printf("CheckBillImport row %d: %v", i, err)
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		rows = append(rows, row)
	}
	c.JSON(http.StatusOK, gin.H{"rows": rows})
}

// checkRow checks one row. It returns an error only when a read fails, because no answer for a row is true without its reads.
func (k *importChecker) checkRow(c *gin.Context, index int, raw json.RawMessage) (importRowResult, error) {
	row := importRowResult{Index: index, Errors: []importIssue{}, Warnings: []importIssue{}}
	request := newSaleRequest()
	if err := json.Unmarshal(raw, &request); err != nil {
		row.Errors = append(row.Errors, importIssue{Code: importInvalidRequest, Message: "the row is not a sale request: " + err.Error()})
		return row, nil
	}
	// The fingerprint is taken before saleProducts clears the manual product ids, so it is the fingerprint that the sale stores.
	fingerprint, err := saleFingerprint(&request)
	if err != nil {
		return row, err
	}
	row.RequestFingerprint = &fingerprint
	row.Errors = append(row.Errors, k.saleIssues(&request)...)
	issues, err := k.storeBranchIssues(c, &request)
	if err != nil {
		return row, err
	}
	row.Errors = append(row.Errors, issues...)
	row.Errors = append(row.Errors, storedScaleIssues(&request)...)
	if request.EffectiveDate != nil {
		errs, warns := dateIssues(savedEffectiveDate(*request.EffectiveDate), k.now)
		row.Errors = append(row.Errors, errs...)
		row.Warnings = append(row.Warnings, warns...)
	}
	if request.BranchID != 0 {
		issues, err := k.branchIssues(c, request.BranchID)
		if err != nil {
			return row, err
		}
		row.Errors = append(row.Errors, issues...)
	}
	if request.PaidAmount.IsPositive() {
		if err := paymentAboveTotalError(request.PaidAmount, saleTotal(saleProducts(&request))); err != nil {
			row.Errors = append(row.Errors, saleIssue(err))
		}
	}
	taken := map[uint64]decimal.Decimal{}
	if request.State > 0 && k.stock != model.StockEnforcementDisable {
		issues, err := k.stockIssues(c, request.Products, taken)
		if err != nil {
			return row, err
		}
		row.Errors = append(row.Errors, issues...)
	}
	if request.BranchID != 0 {
		id, err := k.duplicate(c, request.BranchID, fingerprint)
		if err != nil {
			return row, err
		}
		if id != nil {
			row.DuplicateOf = id
			row.Warnings = append(row.Warnings, importIssue{Code: importDuplicateSale, Message: fmt.Sprintf("this sale matches bill %d on this branch", *id)})
		}
	}
	if first, ok := k.seen[fingerprint]; ok {
		row.Warnings = append(row.Warnings, importIssue{Code: importDuplicateInBatch, Message: fmt.Sprintf("this sale has the same content as row %d", first)})
	} else {
		k.seen[fingerprint] = index
	}
	row.Valid = len(row.Errors) == 0
	if row.Valid {
		for productID, quantity := range taken {
			k.taken[productID] = k.taken[productID].Add(quantity)
		}
	}
	return row, nil
}

// saleIssues checks a row with the rules of the sale path: the binding of the request, the final state, the paid amount, the lines as
// the sale path refuses them, and the caller's stores. It reads nothing.
func (k *importChecker) saleIssues(request *model.AddBillRequest) []importIssue {
	var issues []importIssue
	if err := binding.Validator.ValidateStruct(request); err != nil {
		issues = append(issues, importIssue{Code: importInvalidRequest, Message: err.Error()})
	}
	if request.State <= 0 {
		issues = append(issues, importIssue{Code: importNotFinal, Message: fmt.Sprintf("the sale is not final (state %d)", request.State)})
	}
	for _, err := range []error{paidAmountError(request.PaidAmount), verifyFinalSale(request)} {
		if err != nil {
			issues = append(issues, saleIssue(err))
		}
	}
	if !storeAllowed(k.storeIDs, request.StoreId) {
		issues = append(issues, importIssue{Code: importStoreNotAllowed, Message: fmt.Sprintf("store %d is not one of the stores you can sell from", request.StoreId)})
	}
	return issues
}

// storeBranchIssues checks that the store of a row belongs to the row's branch, as the sale path checks it before it writes a sale. A row
// with no branch is not read: the request refuses it on its own rules.
func (k *importChecker) storeBranchIssues(c *gin.Context, request *model.AddBillRequest) ([]importIssue, error) {
	if request.BranchID == 0 {
		return nil, nil
	}
	storeBranch, err := k.h.queries.GetStoreBranchID(c.Request.Context(), request.StoreId)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if storeBranch == nil || *storeBranch != request.BranchID {
		return []importIssue{{Code: importStoreNotInBranch, Message: "store does not belong to the branch"}}, nil
	}
	return nil, nil
}

// storedScaleIssues checks the values of a sale as bill_product stores them: a price at two decimals, a quantity at three, and the
// maintenance cost as a line's price. bill_product refuses a line that would store as zero (chk_price, chk_quantity). The sale path does
// not check this before it writes, so the database refuses such a sale with its own error. The check reports the row here, so that it is
// invalid before anything is written. A negative price, or a quantity that is not positive, is refused by verifyFinalSale, so it is not
// reported again.
func storedScaleIssues(request *model.AddBillRequest) []importIssue {
	var issues []importIssue
	if request.MaintenanceCost.IsPositive() && !request.MaintenanceCost.Round(2).IsPositive() {
		issues = append(issues, importIssue{Code: "line_price", Message: "maintenance_cost must be at least 0.01"})
	}
	for i, line := range slices.Concat(request.Products, request.ManualProducts) {
		if line.Quantity.IsPositive() && !line.Quantity.Round(3).IsPositive() {
			issues = append(issues, importIssue{Code: "line_quantity", Message: fmt.Sprintf("line %d: quantity must be positive", i+1)})
		}
		if !line.Price.IsNegative() && !line.Price.Round(2).IsPositive() {
			issues = append(issues, importIssue{Code: "line_price", Message: fmt.Sprintf("line %d: price must be at least 0.01", i+1)})
		}
	}
	return issues
}

// saleIssue is the issue of a refusal of the sale path, with the code of the refusal.
func saleIssue(err error) importIssue {
	var refusal *saleRefusal
	if errors.As(err, &refusal) {
		return importIssue{Code: refusal.code, Message: refusal.message}
	}
	return importIssue{Code: importInvalidRequest, Message: err.Error()}
}

// branchIssues checks the branch of a row with what the daemon reads for it. The branch must exist, with a seller VAT and an EGS id,
// and the daemon must run it. The first two are the checks of verifyBill that the backend can read. The last is the daemon's rule
// for running a branch, which the daemon applies at its start and at each onboarding.
func (k *importChecker) branchIssues(c *gin.Context, branchID uint32) ([]importIssue, error) {
	branch, err := k.branch(c, branchID)
	if err != nil {
		return nil, err
	}
	if !branch.found {
		return []importIssue{{Code: importBranchNotFound, Message: fmt.Sprintf("branch %d does not exist", branchID)}}, nil
	}
	var issues []importIssue
	if branch.vat == "" {
		issues = append(issues, importIssue{Code: importSellerVATMissing, Message: "the branch has no seller VAT number"})
	}
	if branch.egsID == "" {
		issues = append(issues, importIssue{Code: importEGSIDMissing, Message: "the branch has no EGS id: onboard the branch first"})
	}
	if !branch.daemonRuns {
		issues = append(issues, importIssue{Code: importBranchNotRegistered, Message: fmt.Sprintf("branch %d is not active or has no production ZATCA credentials, so the daemon will not report its sales", branchID)})
	}
	return issues, nil
}

// branch returns what the daemon reads for a branch, and reads each branch once per check.
func (k *importChecker) branch(c *gin.Context, branchID uint32) (importBranch, error) {
	if branch, ok := k.branches[branchID]; ok {
		return branch, nil
	}
	var branch importBranch
	row, err := k.h.queries.GetDaemonBranch(c.Request.Context(), branchID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return importBranch{}, err
	default:
		branch = importBranch{found: true, vat: row.SellerVat, egsID: row.EgsID, daemonRuns: row.DaemonRuns.Bool}
	}
	k.branches[branchID] = branch
	return branch, nil
}

// stockIssues checks the catalog lines of a final sale against the stock that is left after the valid rows before it. The sale path
// reads a product's quantity and takes the line from it. A line that the stock cannot cover is refused in enforce mode by the sale
// path's own check, and in warn mode by the database, which does not allow a negative quantity (ch_product_quantity). The check
// refuses it in both modes. taken collects what this row would take.
func (k *importChecker) stockIssues(c *gin.Context, lines []model.BillProduct, taken map[uint64]decimal.Decimal) ([]importIssue, error) {
	var issues []importIssue
	for _, line := range lines {
		if line.ProductId == nil {
			continue
		}
		productID := *line.ProductId
		available, err := k.h.queries.SearchQuantityByID(c.Request.Context(), productID)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			continue // as the sale path does, a product that is not found takes nothing from stock
		case err != nil:
			return nil, err
		}
		left := available.Sub(k.taken[productID]).Sub(taken[productID])
		if left.LessThan(line.Quantity) {
			issues = append(issues, importIssue{Code: importStockShort, Message: fmt.Sprintf("product %d has %s left after the earlier rows, and the sale needs %s", productID, left.String(), line.Quantity.String())})
		}
		taken[productID] = taken[productID].Add(line.Quantity)
	}
	return issues, nil
}

// duplicate returns the id of the bill on a branch with the given request fingerprint, or nil when no bill has it.
func (k *importChecker) duplicate(c *gin.Context, branchID uint32, fingerprint string) (*uint64, error) {
	id, err := k.h.queries.GetBillByRequestFingerprint(c.Request.Context(), db.GetBillByRequestFingerprintParams{
		BranchID:                  &branchID,
		RequestFingerprint:        nullableString(fingerprint),
		RequestFingerprintVersion: identity.FingerprintVersion,
	})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, err
	}
	return &id, nil
}

// saleTotal is the total that bill_product gives the lines of a sale, which a payment is checked against. The sale path writes each
// price and quantity at the scale of its column, and it writes neither a discount nor a VAT rate. A line therefore has no discount and
// the column's default rate. The database test compares this total with the total that the database stores for a saved sale.
func saleTotal(lines []model.BillProduct) decimal.Decimal {
	total := decimal.Zero
	for _, line := range lines {
		beforeVAT := line.Price.Round(2).Mul(line.Quantity.Round(3)).Round(2)
		vat := beforeVAT.Mul(defaultLineVATRate).Round(2)
		total = total.Add(beforeVAT.Add(vat).Round(2))
	}
	return total
}
