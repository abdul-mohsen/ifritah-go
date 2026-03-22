package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// Settings CRUD — 42 config fields across 7 categories
// ============================================================================
//
// DATABASE TABLE UPDATE: If upgrading from an older schema, run this migration
// to ensure all 42 settings keys exist with proper defaults:
//
//   -- Add missing settings keys (idempotent — INSERT IGNORE skips existing)
//   INSERT IGNORE INTO settings (setting_key, value, description) VALUES
//     -- Section 2: Invoice & Tax (new keys)
//     ('payment_terms',            '30',      'Default payment terms in days'),
//     -- Section 4: Appearance (new category)
//     ('theme',                    'light',   'UI theme: light or dark'),
//     ('language',                 'ar',      'UI language: ar or en'),
//     ('date_format',              'dd/mm/yyyy', 'Date display format'),
//     ('number_format',            'en',      'Number format locale: en or ar'),
//     -- Section 5: Notifications (new category)
//     ('notif_invoices',           'true',    'Notify on invoice events'),
//     ('notif_stock',              'true',    'Notify on low stock alerts'),
//     ('notif_payments',           'false',   'Notify on payment events'),
//     ('notif_orders',             'false',   'Notify on new order events'),
//     ('notif_session',            'true',    'Notify on session/login events'),
//     -- Section 6: Security (new keys if missing)
//     ('session_duration',         '60',      'Session timeout in minutes'),
//     ('max_login_attempts',       '5',       'Max failed login attempts'),
//     ('require_strong_password',  'true',    'Enforce strong password policy'),
//     ('auto_logout_inactive',     'true',    'Auto-logout on inactivity'),
//     -- Section 7: Inventory (new keys if missing)
//     ('default_unit',             'piece',   'Default unit of measure'),
//     ('track_inventory',          'true',    'Enable inventory tracking'),
//     ('allow_negative_stock',     'false',   'Allow stock below zero'),
//     ('show_cost_price',          'false',   'Show cost price in product lists'),
//     -- Other
//     ('company_description',      '',        'Company description text'),
//     ('invoice_footer',           '',        'Text printed at bottom of invoices'),
//     ('show_vat_breakdown',       'true',    'Show VAT breakdown on invoices'),
//     ('auto_calculate_vat',       'true',    'Auto-calculate VAT on line items'),
//     ('prices_include_vat',       'false',   'Prices entered are VAT-inclusive'),
//     ('paper_size',               'A4',      'Default print paper size'),
//     ('print_copies',             '1',       'Default number of print copies'),
//     ('show_logo_print',          'true',    'Show logo on printed invoices'),
//     ('show_company_info_print',  'true',    'Show company info on prints'),
//     ('show_qr_print',            'true',    'Show QR code on printed invoices'),
//     ('show_bank_details',        'false',   'Show bank details on invoices'),
//     ('bank_details',             '',        'Bank account details text');
//
// Full settings table schema (if creating from scratch):
//
//   CREATE TABLE IF NOT EXISTS settings (
//     id          INT UNSIGNED    NOT NULL AUTO_INCREMENT,
//     setting_key VARCHAR(100)    NOT NULL,
//     value       TEXT            NULL,
//     description VARCHAR(255)    NULL,
//     updated_by  INT UNSIGNED    NULL,
//     updated_at  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
//     PRIMARY KEY (id),
//     UNIQUE KEY uq_settings_key (setting_key),
//     CONSTRAINT fk_settings_user FOREIGN KEY (updated_by)
//       REFERENCES users(id) ON DELETE SET NULL ON UPDATE CASCADE
//   ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
//
// ============================================================================

// GetSettings returns all settings grouped by category.
// GET /api/v2/settings
//
// Response (7 categories, 42 keys total):
//
//	{
//	  "data": {
//	    "company":       {"company_name": "...", "company_vat": "...", ...},          // 9 keys
//	    "invoice":       {"currency": "SAR", "vat_rate": "15", ...},                 // 12 keys
//	    "print":         {"paper_size": "A4", ...},                                  // 7 keys
//	    "appearance":    {"theme": "light", "language": "ar", ...},                  // 4 keys
//	    "notifications": {"notif_invoices": "true", ...},                            // 5 keys
//	    "security":      {"session_duration": "60", ...},                            // 4 keys
//	    "inventory":     {"low_stock_threshold": "5", ...}                           // 6 keys
//	  }
//	}
func (h *handler) GetSettings(c *gin.Context) {
	rows, err := h.DB.Query("SELECT setting_key, COALESCE(value,'') FROM settings ORDER BY setting_key")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to fetch settings"})
		return
	}
	defer rows.Close()

	// Categorize by prefix — 7 categories matching frontend sections
	company := make(map[string]string)
	invoice := make(map[string]string)
	print_ := make(map[string]string)
	appearance := make(map[string]string)
	notifications := make(map[string]string)
	security := make(map[string]string)
	inventory := make(map[string]string)

	categoryMap := map[string]map[string]string{
		// ── Section 1: Company Info (9 keys) ──
		"company_name":        company,
		"company_name_en":     company,
		"company_vat":         company,
		"company_cr":          company,
		"company_address":     company,
		"company_phone":       company,
		"company_email":       company,
		"company_logo_url":    company,
		"company_description": company,

		// ── Section 2: Invoice & Tax (12 keys) ──
		"currency":           invoice,
		"vat_rate":           invoice,
		"invoice_prefix":     invoice,
		"pb_prefix":          invoice,
		"order_prefix":       invoice,
		"credit_prefix":      invoice,
		"payment_terms":      invoice,
		"invoice_due_days":   invoice,
		"invoice_footer":     invoice,
		"show_vat_breakdown": invoice,
		"auto_calculate_vat": invoice,
		"prices_include_vat": invoice,

		// ── Section 3: Print & PDF (7 keys) ──
		"paper_size":              print_,
		"print_copies":            print_,
		"show_logo_print":         print_,
		"show_company_info_print": print_,
		"show_qr_print":           print_,
		"show_bank_details":       print_,
		"bank_details":            print_,

		// ── Section 4: Appearance (4 keys) ──
		"theme":         appearance,
		"language":      appearance,
		"date_format":   appearance,
		"number_format": appearance,

		// ── Section 5: Notifications (5 keys) ──
		"notif_invoices": notifications,
		"notif_stock":    notifications,
		"notif_payments": notifications,
		"notif_orders":   notifications,
		"notif_session":  notifications,

		// ── Section 6: Security & Sessions (4 keys) ──
		"session_duration":        security,
		"max_login_attempts":      security,
		"require_strong_password": security,
		"auto_logout_inactive":    security,

		// ── Section 7: Inventory & Products (6 keys) ──
		"low_stock_threshold":  inventory,
		"default_unit":         inventory,
		"track_inventory":      inventory,
		"allow_negative_stock": inventory,
		"show_cost_price":      inventory,
		"pagination_per_page":  inventory,
	}

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		if cat, ok := categoryMap[key]; ok {
			cat[key] = value
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"company":       company,
			"invoice":       invoice,
			"print":         print_,
			"appearance":    appearance,
			"notifications": notifications,
			"security":      security,
			"inventory":     inventory,
		},
	})
}

// UpdateSettings updates settings for one category.
// PUT /api/v2/settings
//
// Request:
//
//	{
//	  "category": "company",
//	  "settings": {"company_name": "New Name", "company_vat": "123456789012345"}
//	}
//
// Valid categories: company, invoice, print, appearance, notifications, security, inventory
//
// Response: {"detail": "success"}
func (h *handler) UpdateSettings(c *gin.Context) {
	var req struct {
		Category string            `json:"category" binding:"required"`
		Settings map[string]string `json:"settings" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request"})
		return
	}

	// Whitelist keys per category to prevent arbitrary key injection
	allowedKeys := map[string]map[string]bool{
		"company": {
			"company_name": true, "company_name_en": true, "company_vat": true,
			"company_cr": true, "company_address": true, "company_phone": true,
			"company_email": true, "company_logo_url": true, "company_description": true,
		},
		"invoice": {
			"currency": true, "vat_rate": true, "invoice_prefix": true,
			"pb_prefix": true, "order_prefix": true, "credit_prefix": true,
			"payment_terms": true, "invoice_due_days": true, "invoice_footer": true,
			"show_vat_breakdown": true, "auto_calculate_vat": true, "prices_include_vat": true,
		},
		"print": {
			"paper_size": true, "print_copies": true, "show_logo_print": true,
			"show_company_info_print": true, "show_qr_print": true,
			"show_bank_details": true, "bank_details": true,
		},
		"appearance": {
			"theme": true, "language": true, "date_format": true, "number_format": true,
		},
		"notifications": {
			"notif_invoices": true, "notif_stock": true, "notif_payments": true,
			"notif_orders": true, "notif_session": true,
		},
		"security": {
			"session_duration": true, "max_login_attempts": true,
			"require_strong_password": true, "auto_logout_inactive": true,
		},
		"inventory": {
			"low_stock_threshold": true, "default_unit": true, "track_inventory": true,
			"allow_negative_stock": true, "show_cost_price": true, "pagination_per_page": true,
		},
	}

	allowed, validCategory := allowedKeys[req.Category]
	if !validCategory {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid category"})
		return
	}

	userID := c.GetInt64("userId")

	for key, value := range req.Settings {
		if !allowed[key] {
			continue // silently skip unknown keys
		}
		_, _ = h.DB.Exec(
			`INSERT INTO settings (setting_key, value, updated_by)
			 VALUES (?, ?, ?)
			 ON DUPLICATE KEY UPDATE value = VALUES(value), updated_by = VALUES(updated_by)`,
			key, value, userID,
		)
	}

	c.JSON(http.StatusOK, gin.H{"detail": "success"})
}
