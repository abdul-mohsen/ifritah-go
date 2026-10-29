package handlers

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

const zatcaOnboardingConfigQueryPattern = `SELECT\s+COALESCE\(csr_org_identifier`

func TestZatcaOnboardingStateVisibility(t *testing.T) {
	tests := []struct {
		name        string
		state       string
		step        string
		attempt     int64
		retryCount  int64
		errorCode   *string
		errorDetail *string
		completedAt *time.Time
	}{
		{name: "queued", state: "queued", step: "queued", attempt: 1},
		{name: "compliance", state: "compliance", step: "compliance", attempt: 1},
		{name: "production", state: "production", step: "production", attempt: 1},
		{name: "done", state: "done", step: "done", attempt: 1, completedAt: timePtr(time.Date(2026, 10, 29, 19, 0, 0, 0, time.UTC))},
		{
			name:        "failed",
			state:       "failed",
			step:        "compliance",
			attempt:     2,
			retryCount:  3,
			errorCode:   stringPtr("ZATCA_TIMEOUT"),
			errorDetail: stringPtr("compliance request timed out"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, mock, cleanup := newMockHandler(t)
			defer cleanup()

			mock.ExpectQuery(zatcaOnboardingConfigQueryPattern).
				WithArgs(uint32(42)).
				WillReturnRows(zatcaOnboardingConfigRows(
					tt.state,
					tt.step,
					tt.attempt,
					tt.retryCount,
					tt.errorCode,
					tt.errorDetail,
					tt.completedAt,
				))

			w := runHandlerRequest(
				t,
				h.GetBranchZatcaConfig,
				http.MethodGet,
				"/api/v2/branch/42/zatca",
				"",
				gin.Params{{Key: "id", Value: "42"}},
				false,
			)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
			}
			body := w.Body.String()
			for _, expected := range []string{
				`"onboard_state":"` + tt.state + `"`,
				`"onboard_step":"` + tt.step + `"`,
				`"onboard_attempt":`,
				`"onboard_retry_count":`,
			} {
				if !strings.Contains(body, expected) {
					t.Errorf("response missing %q: %s", expected, body)
				}
			}
			if tt.errorCode != nil && !strings.Contains(body, *tt.errorCode) {
				t.Errorf("response missing error code %q: %s", *tt.errorCode, body)
			}
			if tt.errorDetail != nil && !strings.Contains(body, *tt.errorDetail) {
				t.Errorf("response missing error detail %q: %s", *tt.errorDetail, body)
			}
			if tt.completedAt != nil && !strings.Contains(body, tt.completedAt.Format(time.RFC3339)) {
				t.Errorf("response missing completion timestamp %q: %s", tt.completedAt.Format(time.RFC3339), body)
			}
			assertMockExpectations(t, mock)
		})
	}
}

func TestOnboardBranchZatcaPersistsQueuedAttemptBeforePublishing(t *testing.T) {
	h, mock, cleanup := newMockHandler(t)
	defer cleanup()

	mock.ExpectQuery(zatcaOnboardingConfigQueryPattern).
		WithArgs(uint32(42)).
		WillReturnRows(zatcaOnboardingConfigRows(
			"not_started",
			"",
			0,
			0,
			nil,
			nil,
			nil,
		))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE\s+branch_zatca_config`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT\s+INTO\s+zatca_onboarding_attempt`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

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
	for _, expected := range []string{`"onboard_state":"queued"`, `"onboard_step":"queued"`} {
		if !strings.Contains(w.Body.String(), expected) {
			t.Errorf("queued response missing %q: %s", expected, w.Body.String())
		}
	}
	if publisher.onboardCalls != 1 || publisher.otp != "123456" {
		t.Fatalf("publisher = %#v, want one call with trimmed OTP", publisher)
	}
	assertMockExpectations(t, mock)
}

func TestOnboardBranchZatcaPersistsPublisherFailure(t *testing.T) {
	h, mock, cleanup := newMockHandler(t)
	defer cleanup()

	mock.ExpectQuery(zatcaOnboardingConfigQueryPattern).
		WithArgs(uint32(42)).
		WillReturnRows(zatcaOnboardingConfigRows(
			"not_started",
			"",
			0,
			0,
			nil,
			nil,
			nil,
		))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE\s+branch_zatca_config`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT\s+INTO\s+zatca_onboarding_attempt`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectExec(`UPDATE\s+branch_zatca_config`).
		WithArgs("publisher_error", "nats unavailable", "nats unavailable", uint32(42), uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE\s+zatca_onboarding_attempt`).
		WithArgs("publisher_error", "nats unavailable", uint32(42), uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	publisher := &failingZatcaPublisher{err: errors.New("nats unavailable")}
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

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusInternalServerError, w.Body.String())
	}
	assertMockExpectations(t, mock)
}

func TestOnboardBranchZatcaDoesNotDuplicateActiveAttempt(t *testing.T) {
	h, mock, cleanup := newMockHandler(t)
	defer cleanup()

	mock.ExpectQuery(zatcaOnboardingConfigQueryPattern).
		WithArgs(uint32(42)).
		WillReturnRows(zatcaOnboardingConfigRows(
			"compliance",
			"compliance",
			2,
			1,
			nil,
			nil,
			nil,
		))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE\s+branch_zatca_config`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

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

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusConflict, w.Body.String())
	}
	if publisher.onboardCalls != 0 {
		t.Fatalf("active attempt was published %d time(s)", publisher.onboardCalls)
	}
	assertMockExpectations(t, mock)
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func stringPtr(value string) *string {
	return &value
}

type failingZatcaPublisher struct {
	err error
}

func (p *failingZatcaPublisher) SubmitBill(_, _ int64) error {
	return nil
}

func (p *failingZatcaPublisher) SubmitCredit(_, _ int64) error {
	return nil
}

func (p *failingZatcaPublisher) SubmitDebit(_, _ int64) error {
	return nil
}

func (p *failingZatcaPublisher) OnboadBranch(_ int64, _ string) error {
	return p.err
}

func zatcaOnboardingConfigRows(
	state, step string,
	attempt, retryCount int64,
	errorCode, errorDetail *string,
	completedAt *time.Time,
) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"orgid", "orgunit", "orgname", "csrcountry", "csrloc", "bizcat",
		"vat", "crn", "street", "building", "district", "postal",
		"csrlen", "prodlen", "zatca_registered_at", "zatca_status",
		"zatca_onboarded_at", "name",
		"onboard_state", "onboard_step", "onboard_attempt", "onboard_retry_count",
		"onboard_error_code", "onboard_error", "onboard_queued_at",
		"onboard_started_at", "onboard_completed_at", "last_error", "last_attempt_at",
	}).AddRow(
		"300000000000003", "Main", "ACME LLC", "SA", "RRRD2929",
		"Retail", "300000000000003", "7000000000", "King Fahd Rd",
		"1234", "Olaya", "12345",
		int32(0), int32(0), nil, int8(3), nil, "Main Branch",
		state, step, attempt, retryCount, errorCode, errorDetail,
		time.Date(2026, 10, 29, 18, 0, 0, 0, time.UTC), nil, completedAt, errorDetail,
		time.Date(2026, 10, 29, 18, 0, 0, 0, time.UTC),
	)
}
