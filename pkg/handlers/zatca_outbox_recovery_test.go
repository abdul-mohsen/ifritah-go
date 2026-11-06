package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newOutboxRecoveryContext(t *testing.T, role, body string) (*handler, *gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/zatca/outbox/event/replay", strings.NewReader(body))
	c.Params = gin.Params{{Key: "event_id", Value: "1d2b3c4d-0000-4000-8000-00000000000a"}}
	c.Set("user_role", role)
	return &handler{}, c, recorder
}

func TestZatcaOutboxReplayRequiresAdmin(t *testing.T) {
	h, c, recorder := newOutboxRecoveryContext(t, RoleManager, `{"reason":"retry after daemon restart"}`)

	h.ReplayZatcaOutbox(c)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d; want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestZatcaOutboxReplayRequiresAuthenticatedOperator(t *testing.T) {
	h, c, recorder := newOutboxRecoveryContext(t, RoleAdmin, `{"reason":"retry after daemon restart"}`)
	c.Request.Header.Set("Idempotency-Key", "replay-handler-001")

	h.ReplayZatcaOutbox(c)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d; want %d", recorder.Code, http.StatusUnauthorized)
	}
}
