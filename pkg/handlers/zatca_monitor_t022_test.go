package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"
)

func TestT022MonitorSubmissionsHandlerUsesSeededProviderAndRetryRows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	submittedAt := time.Date(2026, 10, 29, 18, 0, 0, 0, time.UTC)
	lastAttemptAt := time.Date(2026, 10, 29, 18, 1, 0, 0, time.UTC)
	nextRetryAt := time.Date(2026, 10, 29, 20, 0, 0, 0, time.UTC)
	outcomeAt := time.Date(2026, 10, 29, 18, 2, 0, 0, time.UTC)
	pendingSince := time.Date(2026, 10, 29, 16, 0, 0, 0, time.UTC)
	requestPath := "zatca/request/41.xml"
	clearedPath := "zatca/cleared/41.xml"

	mock.ExpectQuery("FROM zatca_submission zs").
		WillReturnRows(sqlmock.NewRows([]string{
			"invoice_id",
			"sequence_number",
			"branch_id",
			"branch_name",
			"document_id",
			"outcome_state",
			"provider_code",
			"provider_warning_code",
			"provider_warning_message",
			"provider_reference",
			"legacy_status",
			"rejection_code",
			"rejection_msg",
			"submitted_at",
			"retry_count",
			"last_attempt_at",
			"next_retry_at",
			"request_xml_path",
			"cleared_xml_path",
			"outcome_at",
			"attempt_number",
			"retry_state",
			"retry_error_code",
			"retry_error_message",
			"pending_since",
			"pending_age_seconds",
		}).
			AddRow(
				uint64(41),
				uint64(20260041),
				uint32(7),
				"Main",
				"bill:41",
				"accepted_with_warning",
				"202",
				"W-UBL-01",
				"Minor validation warning",
				"provider-ref-41",
				int8(4),
				nil,
				nil,
				submittedAt,
				int32(2),
				lastAttemptAt,
				nextRetryAt,
				requestPath,
				clearedPath,
				outcomeAt,
				uint32(2),
				"accepted_with_warning",
				nil,
				nil,
				nil,
				nil,
			).
			AddRow(
				uint64(42),
				uint64(20260042),
				uint32(7),
				"Main",
				"bill:42",
				"pending",
				nil,
				nil,
				nil,
				nil,
				int8(1),
				nil,
				nil,
				submittedAt,
				int32(3),
				lastAttemptAt,
				nextRetryAt,
				nil,
				nil,
				nil,
				uint32(3),
				"pending",
				"TIMEOUT",
				"provider request timed out",
				pendingSince,
				int64(7200),
			))

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/zatca/monitor/submissions?limit=50", bytes.NewReader(nil))
	c.Set("decoded_jwt", &model.Claims{Id: 101, Role: RoleAdmin})

	h := handler{queries: db.New(sqlDB)}
	h.ZatcaMonitorSubmissions(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var response []map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response) != 2 {
		t.Fatalf("got %d submission rows, want 2", len(response))
	}

	if response[0]["state"] != "accepted_with_warning" ||
		response[0]["provider_code"] != "202" ||
		response[0]["provider_warning_code"] != "W-UBL-01" ||
		response[0]["provider_reference"] != "provider-ref-41" ||
		response[0]["retry_count"] != float64(2) ||
		response[0]["attempt_number"] != float64(2) ||
		response[0]["retry_state"] != "accepted_with_warning" {
		t.Fatalf("accepted seeded row was not returned intact: %#v", response[0])
	}

	if response[1]["state"] != "pending" ||
		response[1]["retry_count"] != float64(3) ||
		response[1]["attempt_number"] != float64(3) ||
		response[1]["retry_state"] != "pending" ||
		response[1]["retry_error_code"] != "TIMEOUT" ||
		response[1]["pending_age_seconds"] != float64(7200) ||
		response[1]["provider_code"] != nil ||
		response[1]["provider_reference"] != nil {
		t.Fatalf("pending seeded row was not returned with nullable provider data: %#v", response[1])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestT022MonitorStatsHandlerPassesAuthenticatedTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	mock.ExpectQuery("FROM zatca_submission zs").
		WithArgs(int32(202), int32(202)).
		WillReturnRows(sqlmock.NewRows([]string{
			"total",
			"accepted",
			"accepted_with_warning",
			"rejected",
			"ambiguous",
			"terminal_failure",
			"pending",
			"oldest_pending_at",
			"pending_age_seconds",
		}).AddRow(
			int64(1),
			int64(1),
			int64(0),
			int64(0),
			int64(0),
			int64(0),
			int64(0),
			nil,
			nil,
		))

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/zatca/monitor/stats", nil)
	c.Set("decoded_jwt", &model.Claims{Id: 202, Role: RoleAdmin})

	h := handler{queries: db.New(sqlDB)}
	h.ZatcaMonitorStats(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["total_submitted"] != float64(1) ||
		response["accepted"] != float64(1) ||
		response["pending"] != float64(0) ||
		response["oldest_pending_at"] != nil ||
		response["pending_age_seconds"] != nil {
		t.Fatalf("tenant seeded stats were not returned intact: %#v", response)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}
