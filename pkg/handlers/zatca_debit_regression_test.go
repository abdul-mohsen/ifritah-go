package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

func TestDebitBillCreatesDurableSourceBeforePublication(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}

	defer sqlDB.Close()

	stream := &regressionJetStream{}
	h := New(sqlDB, db.New(sqlDB), newRegressionPublisher(stream))

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows([]string{
			"bill_id",
			"merchant_id",
			"branch_id",
			"sequence_number",
			"egs_id",
		}).AddRow(uint64(99), int32(7), uint32(3), uint64(20260099), "egs-3"))
	mock.ExpectExec("INSERT INTO debit_note").
		WillReturnResult(sqlmock.NewResult(77, 1))
	mock.ExpectExec("UPDATE debit_note").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO zatca_submission").
		WillReturnResult(sqlmock.NewResult(88, 1))
	mock.ExpectQuery("SELECT").
		WithArgs(uint64(77), int32(7), int32(7), uint32(3)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"bill_id",
			"merchant_id",
			"branch_id",
			"sequence_number",
			"egs_id",
			"document_id",
			"stable_document_key",
		}).AddRow(
			uint64(77),
			uint64(99),
			int32(7),
			uint32(3),
			uint64(20260099),
			"egs-3",
			"debit:77",
			"test_tenant:3:debit:77",
		))
	mock.ExpectExec("INSERT INTO zatca_event_outbox").
		WillReturnResult(sqlmock.NewResult(89, 1))
	mock.ExpectCommit()

	ginContext := newDebitTestContext(t, http.MethodPost, "/api/v2/bill/debit",
		`{"bill_id":99,"note":"additional taxable charge"}`)
	ginContext.Set("decoded_jwt", &model.Claims{Id: 7, Username: "admin"})
	h.DebitBill(ginContext)

	if ginContext.Writer.Status() != http.StatusCreated {
		t.Fatalf("status = %d, want %d", ginContext.Writer.Status(), http.StatusCreated)
	}
	if stream.calls != 0 {
		t.Fatalf("publisher calls = %d, want 0 before commit", stream.calls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("debit transaction expectations: %v", err)
	}
}

func TestDebitBillRejectsParentFromAnotherTenant(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	h := New(sqlDB, db.New(sqlDB), newRegressionPublisher(&regressionJetStream{}))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT").WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	c := newDebitTestContext(t, http.MethodPost, "/api/v2/bill/debit",
		`{"bill_id":99,"note":"cross-tenant attempt"}`)
	c.Set("decoded_jwt", &model.Claims{Id: 7, Username: "admin"})
	h.DebitBill(c)

	if c.Writer.Status() != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", c.Writer.Status(), http.StatusBadRequest)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("tenant boundary expectations: %v", err)
	}
}

func TestDebitPublisherEnvelopeCarriesDurableSequenceAndEGS(t *testing.T) {
	publisher := newRegressionPublisher(&regressionJetStream{})
	sequence := uint64(20260099)

	params, err := publisher.buildZatcaOutboxParamsWithContext(
		7,
		zatcaDocDebit,
		77,
		3,
		"",
		"egs-3",
		&sequence,
	)
	if err != nil {
		t.Fatalf("buildZatcaOutboxParamsWithContext: %v", err)
	}

	var message Message
	if err := json.Unmarshal(params.Payload, &message); err != nil {
		t.Fatalf("decode debit envelope: %v", err)
	}
	if message.SourceID != 77 || message.DocumentType != zatcaDocDebit {
		t.Fatalf("source context = %#v, want debit/77", message)
	}
	if message.EGSID != "egs-3" {
		t.Fatalf("egs_id = %q, want egs-3", message.EGSID)
	}
	if message.SequenceNumber == nil || *message.SequenceNumber != sequence {
		t.Fatalf("sequence_number = %v, want %d", message.SequenceNumber, sequence)
	}
	if message.StableDocumentKey != "test_tenant:3:debit:77" {
		t.Fatalf("stable key = %q, want test_tenant:3:debit:77", message.StableDocumentKey)
	}
}

func TestDebitMonitorIncludesSourceAndEGSContext(t *testing.T) {
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
			"document_type",
			"source_id",
			"egs_id",
			"stable_document_key",
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
			uint64(99),
			uint64(20260099),
			uint32(3),
			"Main",
			"debit",
			uint64(77),
			"egs-3",
			"test_tenant:3:debit:77",
			"debit:77",
			"pending",
			nil,
			nil,
			nil,
			nil,
			int8(0),
			nil,
			nil,
			nil,
			int32(0),
			nil,
			nil,
			nil,
			nil,
			nil,
		))

	rows, err := db.New(sqlDB).ListZatcaMonitorSubmissions(
		context.Background(),
		db.ListZatcaMonitorSubmissionsParams{
			PendingFlag: "",
			StateFilter: "",
			StatusCode:  -1,
			BranchID:    debitUint32Ptr(0),
			Limit:       50,
		},
	)
	if err != nil {
		t.Fatalf("debit monitor source context: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d monitor rows, want 1", len(rows))
	}
	row := rows[0]
	if row.DocumentType != "debit" || row.SourceID != 77 {
		t.Fatalf("document identity = %q/%d, want debit/77", row.DocumentType, row.SourceID)
	}
	if row.EgsID == nil || *row.EgsID != "egs-3" {
		t.Fatalf("egs_id = %v, want egs-3", row.EgsID)
	}
	if row.StableDocumentKey == nil || *row.StableDocumentKey != "test_tenant:3:debit:77" {
		t.Fatalf("stable key = %v, want test_tenant:3:debit:77", row.StableDocumentKey)
	}
	if row.SequenceNumber == nil || *row.SequenceNumber != 20260099 {
		t.Fatalf("sequence_number = %v, want 20260099", row.SequenceNumber)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("debit monitor expectations: %v", err)
	}
}

func newDebitTestContext(t *testing.T, method, path, body string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func debitUint32Ptr(v uint32) *uint32 {
	return &v
}
