package db_test

import (
	"context"
	"reflect"
	"testing"

	db "ifritah/web-service-gin/pkg/db/gen"

	"github.com/DATA-DOG/go-sqlmock"
)

// TestZatcaMonitorReturnsProviderOutcome is intentionally added before the
// T003 persistence fix. At the base commit the generated monitor query scans
// only the legacy columns, so the provider outcome row/status below cannot be
// returned and this test fails with a scan-column mismatch. After the schema,
// query, and sqlc contracts are fixed, it verifies the explicit provider state
// and code are present.
func TestZatcaMonitorReturnsProviderOutcome(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	mock.ExpectQuery("FROM zatca_submission").
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
		}).AddRow(
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
			"Minor validation warning",
			nil,
			int32(1),
			nil,
			nil,
			"zatca/request/41.xml",
			"zatca/cleared/41.xml",
			nil,
		))

	rows, err := db.New(sqlDB).ListZatcaMonitorSubmissions(
		context.Background(),
		db.ListZatcaMonitorSubmissionsParams{
			PendingFlag: "",
			StatusCode:  -1,
			BranchID:    uint32Ptr(0),
			Limit:       50,
		},
	)
	if err != nil {
		t.Fatalf("provider outcome row/status is absent or wrong: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d monitor rows, want 1", len(rows))
	}

	row := reflect.ValueOf(rows[0])
	if got := nullableStringField(row, "OutcomeState"); got != "accepted_with_warning" {
		t.Fatalf("outcome state = %q, want %q", got, "accepted_with_warning")
	}
	if got := nullableStringField(row, "ProviderCode"); got != "202" {
		t.Fatalf("provider code = %q, want %q", got, "202")
	}
	if got := nullableStringField(row, "ProviderWarningCode"); got != "W-UBL-01" {
		t.Fatalf("provider warning code = %q, want %q", got, "W-UBL-01")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func uint32Ptr(v uint32) *uint32 {
	return &v
}

func nullableStringField(row reflect.Value, name string) string {
	field := row.FieldByName(name)
	if !field.IsValid() {
		return ""
	}
	if field.Kind() == reflect.Pointer {
		if field.IsNil() {
			return ""
		}
		field = field.Elem()
	}
	if field.Kind() != reflect.String {
		return ""
	}
	return field.String()
}
