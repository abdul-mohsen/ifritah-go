package handlers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

func TestZatcaImportPersistsEvidenceAndDoesNotCreateOutboundState(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	stream := &regressionJetStream{}
	h := New(sqlDB, db.New(sqlDB), newRegressionPublisher(stream))
	body := zatcaImportRequestJSON(t, "upload:ubl-42")

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO zatca_import_job").
		WillReturnResult(sqlmock.NewResult(42, 1))
	mock.ExpectExec("INSERT INTO zatca_import_evidence").
		WillReturnResult(sqlmock.NewResult(43, 1))
	mock.ExpectExec("INSERT INTO zatca_import_validation_error").
		WillReturnResult(sqlmock.NewResult(44, 1))
	mock.ExpectExec("INSERT INTO zatca_import_status_history").
		WillReturnResult(sqlmock.NewResult(45, 1))
	mock.ExpectCommit()

	c := newZatcaImportTestContext(t, http.MethodPost, "/api/v2/zatca/import", body)
	c.Set("decoded_jwt", &model.Claims{Id: 7, Username: "admin"})
	h.CreateZatcaImport(c)

	if c.Writer.Status() != http.StatusCreated {
		t.Fatalf("status = %d, want %d", c.Writer.Status(), http.StatusCreated)
	}
	if stream.calls != 0 {
		t.Fatalf("outbound publisher calls = %d, want 0", stream.calls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("import persistence expectations: %v", err)
	}
}

func TestZatcaImportLifecyclePersistsReviewPostAndRejectSeparately(t *testing.T) {
	t.Run("review then post", func(t *testing.T) {
		sqlDB, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock.New: %v", err)
		}
		defer sqlDB.Close()

		h := New(sqlDB, db.New(sqlDB), newRegressionPublisher(&regressionJetStream{}))

		mock.ExpectBegin()
		mock.ExpectQuery("SELECT").
			WillReturnRows(zatcaImportJobRows("review", 1))
		mock.ExpectExec("INSERT INTO zatca_import_review").
			WillReturnResult(sqlmock.NewResult(51, 1))
		mock.ExpectExec("UPDATE zatca_import_job").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("INSERT INTO zatca_import_status_history").
			WillReturnResult(sqlmock.NewResult(52, 1))
		mock.ExpectCommit()

		review := newZatcaImportTestContext(t, http.MethodPost, "/api/v2/zatca/import/42/review",
			`{"decision":"approve","reason":"evidence checked","expected_workflow_version":1}`)
		review.Params = gin.Params{{Key: "id", Value: "42"}}
		review.Set("decoded_jwt", &model.Claims{Id: 7, Username: "reviewer"})
		h.ReviewZatcaImport(review)
		if review.Writer.Status() != http.StatusOK {
			t.Fatalf("review status = %d, want %d", review.Writer.Status(), http.StatusOK)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("review expectations: %v", err)
		}

		mock.ExpectBegin()
		mock.ExpectQuery("SELECT").
			WillReturnRows(zatcaImportJobRows("approved", 2))
		mock.ExpectExec("UPDATE zatca_import_job").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("INSERT INTO zatca_import_status_history").
			WillReturnResult(sqlmock.NewResult(53, 1))
		mock.ExpectCommit()

		post := newZatcaImportTestContext(t, http.MethodPost, "/api/v2/zatca/import/42/post",
			`{"post_reference":"purchase-intake:42","expected_workflow_version":2}`)
		post.Params = gin.Params{{Key: "id", Value: "42"}}
		post.Set("decoded_jwt", &model.Claims{Id: 7, Username: "poster"})
		h.PostZatcaImport(post)
		if post.Writer.Status() != http.StatusOK {
			t.Fatalf("post status = %d, want %d", post.Writer.Status(), http.StatusOK)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("post expectations: %v", err)
		}
	})

	t.Run("reject", func(t *testing.T) {
		sqlDB, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock.New: %v", err)
		}
		defer sqlDB.Close()

		h := New(sqlDB, db.New(sqlDB), newRegressionPublisher(&regressionJetStream{}))
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT").
			WillReturnRows(zatcaImportJobRows("review", 1))
		mock.ExpectExec("UPDATE zatca_import_job").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("INSERT INTO zatca_import_status_history").
			WillReturnResult(sqlmock.NewResult(61, 1))
		mock.ExpectCommit()

		reject := newZatcaImportTestContext(t, http.MethodPost, "/api/v2/zatca/import/42/reject",
			`{"reason":"signature evidence requires supplier correction","expected_workflow_version":1}`)
		reject.Params = gin.Params{{Key: "id", Value: "42"}}
		reject.Set("decoded_jwt", &model.Claims{Id: 7, Username: "reviewer"})
		h.RejectZatcaImport(reject)
		if reject.Writer.Status() != http.StatusOK {
			t.Fatalf("reject status = %d, want %d", reject.Writer.Status(), http.StatusOK)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("reject expectations: %v", err)
		}
	})
}

func TestGetZatcaImportLoadsImmutableEvidenceWithTenantPredicate(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	h := New(sqlDB, db.New(sqlDB), newRegressionPublisher(&regressionJetStream{}))
	mock.ExpectQuery("SELECT").WithArgs(uint64(42), int32(7)).
		WillReturnRows(zatcaImportJobRows("review", 1))
	mock.ExpectQuery("SELECT").WithArgs(uint64(42), int32(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "import_job_id", "merchant_id", "source_reference",
			"source_original_name", "source_mime_type", "source_provenance",
			"source_content_hash", "source_size", "original_bytes",
			"canonical_data", "parsed_data", "signature_data", "validation_data",
			"result_data", "failure_class", "failure_message", "created_at",
		}).AddRow(
			uint64(43), uint64(42), int32(7), "upload:ubl-42", "supplier.xml",
			"application/xml", "supplier_xml",
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			uint64(4), "<xml>", []byte(`{"invoice":{"number":"42"}}`),
			[]byte(`{"invoice":{"number":"42"}}`), []byte(`{"status":"unverified"}`),
			[]byte(`{"review_required":true}`), []byte(`{"failure_class":"trust_unavailable"}`),
			"trust_unavailable", "trust unavailable",
			time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC),
		))
	mock.ExpectQuery("SELECT").WithArgs(uint64(42), int32(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "import_job_id", "merchant_id", "severity", "error_code",
			"field_key", "message", "details", "created_by", "created_at",
		}).AddRow(
			uint64(44), uint64(42), int32(7), "warning", "trust_unavailable",
			nil, "trust unavailable", []byte(`{}`), int32(7),
			time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC),
		))
	mock.ExpectQuery("SELECT").WithArgs(uint64(42), int32(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "import_job_id", "merchant_id", "decision", "reason",
			"expected_workflow_version", "observed_workflow_version", "created_by", "created_at",
		}))
	mock.ExpectQuery("SELECT").WithArgs(uint64(42), int32(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "import_job_id", "merchant_id", "from_status", "to_status",
			"transition_key", "workflow_version", "reason", "created_by", "created_at",
		}).AddRow(
			uint64(45), uint64(42), int32(7), nil, "review", "create:42", uint64(1),
			"review required", int32(7), time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC),
		))

	c := newZatcaImportTestContext(t, http.MethodGet, "/api/v2/zatca/import/42", "")
	c.Params = gin.Params{{Key: "id", Value: "42"}}
	c.Set("decoded_jwt", &model.Claims{Id: 7, Username: "reviewer"})
	h.GetZatcaImport(c)
	if c.Writer.Status() != http.StatusOK {
		t.Errorf("status = %d, want %d", c.Writer.Status(), http.StatusOK)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("tenant-safe import evidence expectations: %v", err)
	}
}

func zatcaImportRequestJSON(t *testing.T, reference string) string {
	t.Helper()
	source := []byte("<Invoice xmlns=\"urn:oasis:names:specification:ubl:schema:xsd:Invoice-2\"><ID>42</ID></Invoice>")
	request := model.ZatcaImportRequest{
		OriginalBytes: source,
		Source: model.ZatcaImportSource{
			ContentReference: reference,
			Provenance:       "supplier_xml",
			MIMEType:         "application/xml",
			OriginalName:     "supplier.xml",
		},
		Canonical: json.RawMessage(`{"contract":"afrita.zatca.ubl-import","version":"1","invoice":{"number":"42"}}`),
		Parsed:    json.RawMessage(`{"contract":"afrita.zatca.ubl-import","version":"1","invoice":{"number":"42"}}`),
		Validation: model.ZatcaImportValidation{
			WellFormed:      true,
			SchemaStatus:    "schema_unavailable",
			SignatureStatus: "unverified",
			ReviewRequired:  true,
			Issues: []model.ZatcaImportValidationIssue{{
				Code:     "trust_unavailable",
				Message:  "no trusted certificate roots were supplied",
				Severity: "warning",
			}},
		},
		Signature:      model.ZatcaImportSignature{Present: true, Status: "unverified"},
		FailureClass:   "trust_unavailable",
		FailureMessage: "no trusted certificate roots were supplied",
		IdempotencyKey: "t031-import-42",
	}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal import request: %v", err)
	}
	// []byte JSON is already base64; retain this assertion so the fixture
	// remains visibly aligned with the T030 result wire shape.
	if !strings.Contains(string(data), base64.StdEncoding.EncodeToString(source)) {
		t.Fatalf("request does not retain base64 source bytes: %s", data)
	}
	return string(data)
}

func newZatcaImportTestContext(t *testing.T, method, path, body string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func zatcaImportJobRows(status string, version uint64) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "merchant_id", "branch_id", "status", "workflow_version",
		"source_content_hash", "reviewed_by", "reviewed_at", "posted_by",
		"posted_at", "post_reference", "rejected_by", "rejected_at",
		"rejected_reason", "created_by", "created_at", "updated_at",
	}).AddRow(
		uint64(42), int32(7), nil, status, version,
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		nil, nil, nil, nil, nil, nil, nil, nil, int32(7),
		time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC),
	)
}
