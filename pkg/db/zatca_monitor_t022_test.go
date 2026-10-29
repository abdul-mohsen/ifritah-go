package db_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	db "ifritah/web-service-gin/pkg/db/gen"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestT022MonitorStatsUseSeededTenantAggregates(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	pendingSince := time.Date(2026, 10, 29, 17, 49, 26, 0, time.UTC)
	mock.ExpectQuery("FROM zatca_submission zs").
		WithArgs(int32(101), int32(101)).
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
			int64(10),
			int64(2),
			int64(1),
			int64(2),
			int64(1),
			int64(1),
			int64(3),
			pendingSince,
			int64(7200),
		))

	row, err := db.New(sqlDB).GetZatcaMonitorStats(
		context.Background(),
		db.GetZatcaMonitorStatsParams{MerchantID: 101},
	)
	if err != nil {
		t.Fatalf("seeded tenant stats were not read: %v", err)
	}
	if row.Total != 10 || row.Accepted != 2 || row.AcceptedWithWarning != 1 ||
		row.Rejected != 2 || row.Ambiguous != 1 || row.TerminalFailure != 1 ||
		row.Pending != 3 {
		t.Fatalf("stats = %#v, want the seeded tenant aggregate", row)
	}

	if got := monitorField(row, "OldestPendingAt"); !got.IsValid() {
		t.Fatal("oldest pending timestamp was not projected from the seeded rows")
	}
	if got := int64Field(row, "PendingAgeSeconds"); got != 7200 {
		t.Fatalf("pending age = %d, want 7200 from the seeded rows", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestT022MonitorBranchesUseSeededDurableAggregates(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	pendingSince := time.Date(2026, 10, 29, 16, 49, 26, 0, time.UTC)
	lastSubmission := time.Date(2026, 10, 29, 18, 30, 0, 0, time.UTC)
	nextRetry := time.Date(2026, 10, 29, 20, 0, 0, 0, time.UTC)
	mock.ExpectQuery("FROM branches b").
		WithArgs(int32(101), int32(101)).
		WillReturnRows(sqlmock.NewRows([]string{
			"branch_id",
			"branch_name",
			"zatca_status",
			"certificate",
			"today_count",
			"week_total",
			"week_accepted",
			"last_submission_at",
			"total_submitted",
			"accepted",
			"accepted_with_warning",
			"rejected",
			"pending",
			"ambiguous",
			"terminal_failure",
			"oldest_pending_at",
			"pending_age_seconds",
			"pending_retry_count",
			"pending_next_retry_at",
		}).AddRow(
			uint32(7),
			"Main",
			int8(1),
			nil,
			int64(6),
			int64(8),
			int64(3),
			lastSubmission,
			int64(8),
			int64(2),
			int64(1),
			int64(1),
			int64(2),
			int64(1),
			int64(1),
			pendingSince,
			int64(3600),
			int64(3),
			nextRetry,
		))

	rows, err := db.New(sqlDB).ListZatcaMonitorBranches(
		context.Background(),
		db.ListZatcaMonitorBranchesParams{MerchantID: 101},
	)
	if err != nil {
		t.Fatalf("seeded branch aggregates were not read: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d branch rows, want 1 seeded branch row", len(rows))
	}
	row := rows[0]
	if row.BranchID != 7 || row.BranchName != "Main" ||
		row.TodayCount != 6 || row.WeekTotal != 8 || row.WeekAccepted != 3 {
		t.Fatalf("branch compatibility fields = %#v, want seeded values", row)
	}
	if got := int64Field(row, "TotalSubmitted"); got != 8 {
		t.Fatalf("total submitted = %d, want 8 from the seeded rows", got)
	}
	if got := int64Field(row, "Accepted"); got != 2 {
		t.Fatalf("accepted = %d, want 2 from the seeded rows", got)
	}
	if got := int64Field(row, "AcceptedWithWarning"); got != 1 {
		t.Fatalf("accepted with warning = %d, want 1 from the seeded rows", got)
	}
	if got := int64Field(row, "Rejected"); got != 1 ||
		int64Field(row, "Pending") != 2 ||
		int64Field(row, "Ambiguous") != 1 ||
		int64Field(row, "TerminalFailure") != 1 {
		t.Fatalf("branch state counts do not match seeded rows: %#v", row)
	}
	if got := int64Field(row, "PendingAgeSeconds"); got != 3600 {
		t.Fatalf("branch pending age = %d, want 3600", got)
	}
	if got := int64Field(row, "PendingRetryCount"); got != 3 {
		t.Fatalf("branch pending retry count = %d, want 3", got)
	}
	if got := monitorField(row, "PendingNextRetryAt"); !got.IsValid() {
		t.Fatal("branch next retry timestamp was not projected from the seeded rows")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func monitorField(row any, name string) reflect.Value {
	value := reflect.ValueOf(row)
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return reflect.Value{}
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	field := value.FieldByName(name)
	if !field.IsValid() {
		return reflect.Value{}
	}
	for field.IsValid() && (field.Kind() == reflect.Pointer || field.Kind() == reflect.Interface) {
		if field.IsNil() {
			return reflect.Value{}
		}
		field = field.Elem()
	}
	return field
}

func int64Field(row any, name string) int64 {
	field := monitorField(row, name)
	if !field.IsValid() {
		return 0
	}
	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return field.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(field.Uint())
	default:
		return 0
	}
}
