package handlers

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// TestUpdateSettingsPersistsPBPDFRequired is a regression test for a bug
// where the "should the purchase-bill PDF be required" toggle was never
// wired into the backend settings whitelist at all, so it lived only in the
// frontend's in-memory defaults and reset on every rebuild/redeploy.
// settingCategories is the whitelist UpdateSettings checks before writing
// anything to the settings table - if pb_pdf_required isn't in it, the
// value is silently dropped (never reaches UpsertSetting) regardless of
// what the client sends.
func TestUpdateSettingsPersistsPBPDFRequired(t *testing.T) {
	h, mock, cleanup := newPurchaseBillTestHandler(t)
	defer cleanup()

	mock.ExpectExec("INSERT INTO settings").
		WithArgs("pb_pdf_required", "optional", int32(7)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := `{"category":"invoice","settings":{"pb_pdf_required":"optional"}}`
	w := runPurchaseBillRequest(t, h.UpdateSettings, http.MethodPut, "/api/v2/settings", body)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if !containsUpdatedOne(w.Body.String()) {
		t.Fatalf(`expected {"updated":1} (pb_pdf_required must be persisted), got: %s`, w.Body.String())
	}
	assertMockExpectations(t, mock)
}

// TestGetSettingsGroupsPBPDFRequiredUnderInvoice verifies the read side
// exposes the persisted value back to callers under the "invoice" category,
// so the frontend can read the durable value instead of falling back to its
// own hardcoded, process-local default.
func TestGetSettingsGroupsPBPDFRequiredUnderInvoice(t *testing.T) {
	h, mock, cleanup := newPurchaseBillTestHandler(t)
	defer cleanup()

	mock.ExpectQuery("SELECT setting_key, COALESCE").
		WillReturnRows(sqlmock.NewRows([]string{"setting_key", "value"}).
			AddRow("pb_pdf_required", "optional"))

	w := runPurchaseBillRequest(t, h.GetSettings, http.MethodGet, "/api/v2/settings", "")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	body := w.Body.String()
	if !containsInvoicePBPDFRequired(body) {
		t.Fatalf(`expected settings response to include "pb_pdf_required":"optional" under the "invoice" category, got: %s`, body)
	}
	assertMockExpectations(t, mock)
}

func containsUpdatedOne(body string) bool {
	return regexp.MustCompile(`"updated"\s*:\s*1`).MatchString(body)
}

func containsInvoicePBPDFRequired(body string) bool {
	return regexp.MustCompile(`"invoice"\s*:\s*{[^}]*"pb_pdf_required"\s*:\s*"optional"`).MatchString(body)
}

// TestUpdateSettingsPersistsDefaultMarkupPercentage is a regression test for
// the same whitelist-drop bug pattern as pb_pdf_required: the purchase-bill
// selling-price feature reads default_markup_percentage from the "inventory"
// category, so it must be in settingCategories or UpdateSettings silently
// drops it.
func TestUpdateSettingsPersistsDefaultMarkupPercentage(t *testing.T) {
	h, mock, cleanup := newPurchaseBillTestHandler(t)
	defer cleanup()

	mock.ExpectExec("INSERT INTO settings").
		WithArgs("default_markup_percentage", "25", int32(7)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := `{"category":"inventory","settings":{"default_markup_percentage":"25"}}`
	w := runPurchaseBillRequest(t, h.UpdateSettings, http.MethodPut, "/api/v2/settings", body)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if !containsUpdatedOne(w.Body.String()) {
		t.Fatalf(`expected {"updated":1} (default_markup_percentage must be persisted), got: %s`, w.Body.String())
	}
	assertMockExpectations(t, mock)
}
