package handlers

// ============================================================================
// Settings CRUD — Key-value store grouped by category
// ============================================================================
// Copy to: pkg/handlers/handler_settings.go
//
// Endpoints:
//   GET    /api/v2/settings           → GetSettings (all settings grouped)
//   PUT    /api/v2/settings           → UpdateSettings (per category)
//
// DATABASE:
//   - `settings` table (already exists):
//       id INT UNSIGNED PK, setting_key VARCHAR(100) UNIQUE,
//       value TEXT, description VARCHAR(255),
//       updated_by INT, updated_at DATETIME
//
// Settings are organized into 7 categories (42 keys total):
//   1. company (9)    — company_name, company_vat, company_cr, etc.
//   2. invoice (12)   — currency, vat_rate, invoice_prefix, etc.
//   3. print (7)      — paper_size, print_copies, show_logo_print, etc.
//   4. appearance (4) — theme, language, date_format, number_format
//   5. notifications (5) — notif_invoices, notif_stock, etc.
//   6. security (4)   — session_duration, max_login_attempts, etc.
//   7. inventory (6)  — low_stock_threshold, default_unit, etc.
//
// ZATCA config is NOT in settings — it's per-branch in branch_zatca_config.
// Stock enforcement (stock_enforcement key) is read separately by stock handlers.
// ============================================================================

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// settingCategories maps each setting_key to its category name.
// Used by both GetSettings (to group) and UpdateSettings (to whitelist).
var settingCategories = map[string]string{
	// ── Section 1: Company Info (9 keys) ──
	"company_name":        "company",
	"company_name_en":     "company",
	"company_vat":         "company",
	"company_cr":          "company",
	"company_address":     "company",
	"company_phone":       "company",
	"company_email":       "company",
	"company_logo_url":    "company",
	"company_description": "company",

	// ── Section 2: Invoice & Tax (12 keys) ──
	"currency":           "invoice",
	"vat_rate":           "invoice",
	"invoice_prefix":     "invoice",
	"pb_prefix":          "invoice",
	"order_prefix":       "invoice",
	"credit_prefix":      "invoice",
	"payment_terms":      "invoice",
	"invoice_due_days":   "invoice",
	"invoice_footer":     "invoice",
	"show_vat_breakdown": "invoice",
	"auto_calculate_vat": "invoice",
	"prices_include_vat": "invoice",

	// ── Section 3: Print & PDF (7 keys) ──
	"paper_size":              "print",
	"print_copies":            "print",
	"show_logo_print":         "print",
	"show_company_info_print": "print",
	"show_qr_print":           "print",
	"show_bank_details":       "print",
	"bank_details":            "print",

	// ── Section 4: Appearance (4 keys) ──
	"theme":         "appearance",
	"language":      "appearance",
	"date_format":   "appearance",
	"number_format": "appearance",

	// ── Section 5: Notifications (5 keys) ──
	"notif_invoices": "notifications",
	"notif_stock":    "notifications",
	"notif_payments": "notifications",
	"notif_orders":   "notifications",
	"notif_session":  "notifications",

	// ── Section 6: Security & Sessions (4 keys) ──
	"session_duration":        "security",
	"max_login_attempts":      "security",
	"require_strong_password": "security",
	"auto_logout_inactive":    "security",

	// ── Section 7: Inventory & Products (6 keys) ──
	"low_stock_threshold":  "inventory",
	"default_unit":         "inventory",
	"track_inventory":      "inventory",
	"allow_negative_stock": "inventory",
	"show_cost_price":      "inventory",
	"pagination_per_page":  "inventory",

	// ── Stock enforcement (read by stock handlers, editable via inventory) ──
	"stock_enforcement": "inventory",
}

// ── GET /api/v2/settings ────────────────────────────────────────────────────
// Returns all settings grouped by category.
//
// Response:
//
//	{
//	  "data": {
//	    "company":       {"company_name": "...", "company_vat": "...", ...},
//	    "invoice":       {"currency": "SAR", "vat_rate": "15", ...},
//	    "print":         {"paper_size": "A4", ...},
//	    "appearance":    {"theme": "light", "language": "ar", ...},
//	    "notifications": {"notif_invoices": "true", ...},
//	    "security":      {"session_duration": "60", ...},
//	    "inventory":     {"low_stock_threshold": "5", "stock_enforcement": "disable", ...}
//	  }
//	}

func (h *handler) GetSettings(c *gin.Context) {
	rows, err := h.DB.Query("SELECT setting_key, COALESCE(value,'') FROM settings ORDER BY setting_key")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to fetch settings"})
		return
	}
	defer rows.Close()

	// Initialize all category maps
	grouped := map[string]map[string]string{
		"company":       {},
		"invoice":       {},
		"print":         {},
		"appearance":    {},
		"notifications": {},
		"security":      {},
		"inventory":     {},
	}

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		if cat, ok := settingCategories[key]; ok {
			grouped[cat][key] = value
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": grouped})
}

// ── PUT /api/v2/settings ────────────────────────────────────────────────────
// Updates settings for one category.
//
// Request:
//
//	{
//	  "category": "company",
//	  "settings": {"company_name": "عفريته", "company_vat": "123456789012345"}
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

	// Validate category
	validCategories := map[string]bool{
		"company": true, "invoice": true, "print": true,
		"appearance": true, "notifications": true, "security": true, "inventory": true,
	}
	if !validCategories[req.Category] {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid category"})
		return
	}

	userID := GetSessionInfo(c).id

	updated := 0
	for key, value := range req.Settings {
		// Only allow keys that belong to the requested category (whitelist)
		cat, known := settingCategories[key]
		if !known || cat != req.Category {
			continue
		}

		_, err := h.DB.Exec(
			`INSERT INTO settings (setting_key, value, updated_by)
			 VALUES (?, ?, ?)
			 ON DUPLICATE KEY UPDATE value = VALUES(value), updated_by = VALUES(updated_by)`,
			key, value, userID,
		)
		if err == nil {
			updated++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"detail":  "success",
		"updated": updated,
	})
}

// ============================================================================
// Seed SQL — Run once to ensure all 42+ settings keys exist with defaults
// ============================================================================
//
// INSERT IGNORE INTO settings (setting_key, value, description) VALUES
//   -- Company
//   ('company_name',             '',         'اسم الشركة'),
//   ('company_name_en',          '',         'Company name (English)'),
//   ('company_vat',              '',         'الرقم الضريبي'),
//   ('company_cr',               '',         'السجل التجاري'),
//   ('company_address',          '',         'عنوان الشركة'),
//   ('company_phone',            '',         'هاتف الشركة'),
//   ('company_email',            '',         'البريد الإلكتروني'),
//   ('company_logo_url',         '',         'رابط الشعار'),
//   ('company_description',      '',         'وصف الشركة'),
//   -- Invoice & Tax
//   ('currency',                 'SAR',      'العملة'),
//   ('vat_rate',                 '15',       'نسبة الضريبة %'),
//   ('invoice_prefix',           'INV',      'بادئة رقم الفاتورة'),
//   ('pb_prefix',                'PB',       'بادئة فاتورة المشتريات'),
//   ('order_prefix',             'ORD',      'بادئة الطلب'),
//   ('credit_prefix',            'CN',       'بادئة إشعار دائن'),
//   ('payment_terms',            '30',       'شروط الدفع (أيام)'),
//   ('invoice_due_days',         '30',       'مدة استحقاق الفاتورة'),
//   ('invoice_footer',           '',         'نص ذيل الفاتورة'),
//   ('show_vat_breakdown',       'true',     'عرض تفاصيل الضريبة'),
//   ('auto_calculate_vat',       'true',     'حساب الضريبة تلقائياً'),
//   ('prices_include_vat',       'false',    'الأسعار شاملة الضريبة'),
//   -- Print & PDF
//   ('paper_size',               'A4',       'حجم الورق'),
//   ('print_copies',             '1',        'عدد النسخ'),
//   ('show_logo_print',          'true',     'عرض الشعار في الطباعة'),
//   ('show_company_info_print',  'true',     'عرض بيانات الشركة'),
//   ('show_qr_print',            'true',     'عرض رمز QR'),
//   ('show_bank_details',        'false',    'عرض البيانات البنكية'),
//   ('bank_details',             '',         'تفاصيل الحساب البنكي'),
//   -- Appearance
//   ('theme',                    'light',    'المظهر'),
//   ('language',                 'ar',       'اللغة'),
//   ('date_format',              'dd/mm/yyyy','تنسيق التاريخ'),
//   ('number_format',            'en',       'تنسيق الأرقام'),
//   -- Notifications
//   ('notif_invoices',           'true',     'إشعارات الفواتير'),
//   ('notif_stock',              'true',     'إشعارات المخزون'),
//   ('notif_payments',           'false',    'إشعارات الدفع'),
//   ('notif_orders',             'false',    'إشعارات الطلبات'),
//   ('notif_session',            'true',     'إشعارات الجلسات'),
//   -- Security
//   ('session_duration',         '60',       'مدة الجلسة (دقائق)'),
//   ('max_login_attempts',       '5',        'محاولات الدخول القصوى'),
//   ('require_strong_password',  'true',     'فرض كلمة مرور قوية'),
//   ('auto_logout_inactive',     'true',     'تسجيل خروج تلقائي'),
//   -- Inventory
//   ('low_stock_threshold',      '5',        'حد المخزون المنخفض'),
//   ('default_unit',             'piece',    'وحدة القياس الافتراضية'),
//   ('track_inventory',          'true',     'تتبع المخزون'),
//   ('allow_negative_stock',     'false',    'السماح بمخزون سالب'),
//   ('show_cost_price',          'false',    'عرض سعر التكلفة'),
//   ('pagination_per_page',      '20',       'عدد العناصر في الصفحة'),
//   ('stock_enforcement',        'disable',  'وضع إنفاذ المخزون');
//
