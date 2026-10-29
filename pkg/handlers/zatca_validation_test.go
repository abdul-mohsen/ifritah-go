package handlers

import (
	"database/sql"
	"net/http"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

const zatcaConfigQueryPattern = `SELECT\s+COALESCE\(csr_org_identifier`

type recordingZatcaPublisher struct {
	onboardCalls int
	branchID     int64
	otp          string
}

func (p *recordingZatcaPublisher) SubmitBill(_, _ int64) error {
	return nil
}

func (p *recordingZatcaPublisher) SubmitCredit(_, _ int64) error {
	return nil
}

func (p *recordingZatcaPublisher) SubmitDebit(_, _ int64) error {
	return nil
}

func (p *recordingZatcaPublisher) OnboadBranch(branchID int64, otp string) error {
	p.onboardCalls++
	p.branchID = branchID
	p.otp = otp
	return nil
}

func TestOnboardBranchZatcaRejectsNonASCIIDigitsBeforePublishing(t *testing.T) {
	for _, invalidOTP := range []string{"12a456", "１２３４５６", "١٢٣٤٥٦"} {
		t.Run(invalidOTP, func(t *testing.T) {
			h, mock, cleanup := newMockHandler(t)
			defer cleanup()

			publisher := &recordingZatcaPublisher{}
			h.pub = publisher
			w := runHandlerRequest(
				t,
				h.OnboardBranchZatca,
				http.MethodPost,
				"/api/v2/branch/42/zatca/onboard",
				`{"otp":"`+invalidOTP+`"}`,
				gin.Params{{Key: "id", Value: "42"}},
				false,
			)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusBadRequest, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), "otp") || !strings.Contains(w.Body.String(), "ASCII") {
				t.Fatalf("detail should identify the OTP format: %s", w.Body.String())
			}
			if publisher.onboardCalls != 0 {
				t.Fatalf("invalid OTP was published %d time(s)", publisher.onboardCalls)
			}
			assertMockExpectations(t, mock)
		})
	}
}

func TestOnboardBranchZatcaRejectsIncompleteConfigBeforePublishing(t *testing.T) {
	h, mock, cleanup := newMockHandler(t)
	defer cleanup()

	mock.ExpectQuery(zatcaConfigQueryPattern).
		WithArgs(uint32(42)).
		WillReturnRows(zatcaConfigRows(
			"", "", "", "SA", "", "", "", "", "", "", "", "",
		))
	publisher := &recordingZatcaPublisher{}
	h.pub = publisher

	w := runHandlerRequest(
		t,
		h.OnboardBranchZatca,
		http.MethodPost,
		"/api/v2/branch/42/zatca/onboard",
		`{"otp":"123456"}`,
		gin.Params{{Key: "id", Value: "42"}},
		false,
	)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
	body := w.Body.String()
	for _, field := range []string{"csr_org_identifier", "seller_vat", "postal_code"} {
		if !strings.Contains(body, field) {
			t.Errorf("detail missing field %q: %s", field, body)
		}
	}
	if publisher.onboardCalls != 0 {
		t.Fatalf("incomplete config was published %d time(s)", publisher.onboardCalls)
	}
	assertMockExpectations(t, mock)
}

func TestOnboardBranchZatcaPublishesTrimmedOTPAfterValidation(t *testing.T) {
	h, mock, cleanup := newMockHandler(t)
	defer cleanup()

	mock.ExpectQuery(zatcaConfigQueryPattern).
		WithArgs(uint32(42)).
		WillReturnRows(zatcaConfigRows(
			"300000000000003", "Main", "ACME LLC", "SA", "RRRD2929",
			"Retail", "300000000000003", "7000000000", "King Fahd Rd",
			"1234", "Olaya", "12345",
		))
	publisher := &recordingZatcaPublisher{}
	h.pub = publisher

	w := runHandlerRequest(
		t,
		h.OnboardBranchZatca,
		http.MethodPost,
		"/api/v2/branch/42/zatca/onboard",
		"{\"otp\":\" 123456 \\n\"}",
		gin.Params{{Key: "id", Value: "42"}},
		false,
	)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if publisher.onboardCalls != 1 {
		t.Fatalf("publisher calls = %d, want 1", publisher.onboardCalls)
	}
	if publisher.branchID != 42 {
		t.Fatalf("published branch id = %d, want 42", publisher.branchID)
	}
	if publisher.otp != "123456" {
		t.Fatalf("published OTP = %q, want trimmed %q", publisher.otp, "123456")
	}
	assertMockExpectations(t, mock)
}

func TestOnboardBranchZatcaRejectsMissingConfigBeforePublishing(t *testing.T) {
	h, mock, cleanup := newMockHandler(t)
	defer cleanup()

	mock.ExpectQuery(zatcaConfigQueryPattern).
		WithArgs(uint32(42)).
		WillReturnError(sql.ErrNoRows)
	publisher := &recordingZatcaPublisher{}
	h.pub = publisher

	w := runHandlerRequest(
		t,
		h.OnboardBranchZatca,
		http.MethodPost,
		"/api/v2/branch/42/zatca/onboard",
		`{"otp":"123456"}`,
		gin.Params{{Key: "id", Value: "42"}},
		false,
	)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "zatca_config") {
		t.Fatalf("detail should identify missing config: %s", w.Body.String())
	}
	if publisher.onboardCalls != 0 {
		t.Fatalf("missing config was published %d time(s)", publisher.onboardCalls)
	}
	assertMockExpectations(t, mock)
}

func zatcaConfigRows(values ...string) *sqlmock.Rows {
	if len(values) != 12 {
		panic("zatcaConfigRows requires 12 config values")
	}
	return sqlmock.NewRows([]string{
		"orgid", "orgunit", "orgname", "csrcountry", "csrloc", "bizcat",
		"vat", "crn", "street", "building", "district", "postal",
		"csrlen", "prodlen", "zatca_registered_at", "zatca_status",
		"zatca_onboarded_at", "name",
	}).AddRow(
		values[0], values[1], values[2], values[3], values[4], values[5],
		values[6], values[7], values[8], values[9], values[10], values[11],
		int32(0), int32(0), nil, int8(3), nil, "Main Branch",
	)
}
