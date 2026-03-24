package model

// ============================================================================
// Stock Movement Types & Request/Response Models
// ============================================================================
// Copy this file to: pkg/model/stock.go
// ============================================================================

import (
	"time"

	"github.com/shopspring/decimal"
)

// ── Movement Type Constants ─────────────────────────────────────────────────

const (
	MovementTypePurchase       = "purchase"
	MovementTypeSale           = "sale"
	MovementTypeCreditNote     = "credit_note"
	MovementTypeAdjustment     = "adjustment"
	MovementTypeTransferOut    = "transfer_out"
	MovementTypeTransferIn     = "transfer_in"
	MovementTypeInitialBalance = "initial_balance"
	MovementTypeDeletion       = "deletion"
)

const (
	ReferenceTypePurchaseBill = "purchase_bill"
	ReferenceTypeBill         = "bill"
	ReferenceTypeCreditNote   = "credit_note"
	ReferenceTypeTransfer     = "transfer"
	ReferenceTypeManual       = "manual"
)

// ── Stock Enforcement Modes ─────────────────────────────────────────────────

const (
	StockEnforcementDisable = "disable"
	StockEnforcementWarn    = "warn"
	StockEnforcementEnforce = "enforce"
)

// ── Stock Movement (DB row) ─────────────────────────────────────────────────

type StockMovement struct {
	ID            int64           `json:"id"`
	ProductID     int32           `json:"product_id"`
	StoreID       int32           `json:"store_id"`
	Quantity      decimal.Decimal `json:"quantity"`
	MovementType  string          `json:"movement_type"`
	ReferenceType *string         `json:"reference_type"`
	ReferenceID   *int32          `json:"reference_id"`
	ItemID        *int32          `json:"item_id"`
	Reason        *string         `json:"reason"`
	Note          *string         `json:"note"`
	CreatedBy     *int32          `json:"created_by"`
	CreatedAt     time.Time       `json:"created_at"`
}

// ── Stock Adjust Request ────────────────────────────────────────────────────
// POST /api/v2/stock/adjust
// Used for manual stock corrections (damaged, lost, count_correction, etc.)

type StockAdjustRequest struct {
	ProductID      int32           `json:"product_id" binding:"required"`
	QuantityChange decimal.Decimal `json:"quantity_change" binding:"required"`
	Reason         string          `json:"reason" binding:"required,oneof=damaged lost expired returned_supplier count_correction other"`
	Note           string          `json:"note"`
}

// ── Stock Check Request/Response ────────────────────────────────────────────
// POST /api/v2/stock/check
// Frontend calls this before bill submission when enforce/warn is on.

type StockCheckRequest struct {
	Items []StockCheckItem `json:"items" binding:"required,dive"`
}

type StockCheckItem struct {
	ProductID int32           `json:"product_id" binding:"required"`
	Quantity  decimal.Decimal `json:"quantity" binding:"required"`
}

type StockCheckResponse struct {
	Enforcement   string             `json:"enforcement"`
	Items         []StockCheckResult `json:"items"`
	AllSufficient bool               `json:"all_sufficient"`
}

type StockCheckResult struct {
	ProductID  int32           `json:"product_id"`
	StoreID    int32           `json:"store_id"`
	Available  decimal.Decimal `json:"available"`
	Requested  decimal.Decimal `json:"requested"`
	Sufficient bool            `json:"sufficient"`
}

// ── Movement History Response ───────────────────────────────────────────────
// GET /api/v2/stock/movements/:product_id

type StockMovementResponse struct {
	ID            int64   `json:"id"`
	ProductID     int32   `json:"product_id"`
	StoreID       int32   `json:"store_id"`
	Quantity      string  `json:"quantity"`
	MovementType  string  `json:"movement_type"`
	ReferenceType *string `json:"reference_type"`
	ReferenceID   *int32  `json:"reference_id"`
	ItemID        *int32  `json:"item_id"`
	Reason        *string `json:"reason"`
	Note          *string `json:"note"`
	CreatedBy     *int32  `json:"created_by"`
	CreatedAt     string  `json:"created_at"`
}

// ── Enforcement Response ────────────────────────────────────────────────────
// GET /api/v2/stock/enforcement

type StockEnforcementResponse struct {
	Mode string `json:"mode"`
}
