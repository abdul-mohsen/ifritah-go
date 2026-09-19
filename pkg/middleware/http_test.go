package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"ifritah/web-service-gin/pkg/logging"

	"github.com/gin-gonic/gin"
)

func TestRequestIDResponseAndCompletionSchema(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	logger := logging.New(logging.Config{Level: slog.LevelInfo, Format: "json"}, &output)

	router := gin.New()
	router.Use(RequestLogging(Config{
		Logger:        logger,
		ServerContext: logging.ServerContext{Tenant: "tenant-a", CompanyID: "7"},
	}))
	router.GET("/items/:id", func(c *gin.Context) {
		// A plain Gin value is not trusted authentication context.
		c.Set("user_id", int64(99))
		c.Status(http.StatusCreated)
	})

	request := httptest.NewRequest(http.MethodGet, "/items/42", nil)
	request.Header.Set(logging.RequestIDHeader, "invalid request id")
	request.Header.Set("X-Tenant-ID", "attacker-tenant")
	request.Header.Set("X-Company-ID", "999")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	requestID := response.Header().Get(logging.RequestIDHeader)
	if !logging.ValidRequestID(requestID) {
		t.Fatalf("response request ID %q is invalid", requestID)
	}
	if requestID == "invalid request id" {
		t.Fatal("invalid request ID was accepted")
	}

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode completion log: %v; output=%s", err, output.String())
	}
	if record["msg"] != "http.request.completed" {
		t.Fatalf("msg = %v", record["msg"])
	}
	if record["request_id"] != requestID {
		t.Fatalf("request_id = %v, want %q", record["request_id"], requestID)
	}
	if record["method"] != http.MethodGet || record["route"] != "/items/:id" {
		t.Fatalf("method/route = %v/%v", record["method"], record["route"])
	}
	if record["status"] != float64(http.StatusCreated) {
		t.Fatalf("status = %v", record["status"])
	}
	if _, ok := record["duration_ms"]; !ok {
		t.Fatal("duration_ms missing")
	}
	if record["tenant"] != "tenant-a" || record["company_id"] != "7" {
		t.Fatalf("server context = tenant %v/company %v", record["tenant"], record["company_id"])
	}
	if record["tenant_id"] != "tenant-a" {
		t.Fatalf("tenant_id = %v, want tenant-a", record["tenant_id"])
	}
	if _, ok := record["user_id"]; ok {
		t.Fatalf("untrusted user ID leaked into completion log: %v", record["user_id"])
	}
}

func TestTrustedUserAndValidRequestIDAreLogged(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	logger := logging.New(logging.Config{Level: slog.LevelInfo, Format: "json"}, &output)

	router := gin.New()
	router.Use(RequestID())
	router.Use(RequestCompletion(Config{Logger: logger}))
	router.GET("/trusted", func(c *gin.Context) {
		c.Request = c.Request.WithContext(
			logging.WithTrustedUserID(c.Request.Context(), 42),
		)
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/trusted", nil)
	request.Header.Set(logging.RequestIDHeader, "req-known")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if got := response.Header().Get(logging.RequestIDHeader); got != "req-known" {
		t.Fatalf("request ID = %q, want req-known", got)
	}
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode completion log: %v", err)
	}
	if record["user_id"] != float64(42) {
		t.Fatalf("user_id = %v, want 42", record["user_id"])
	}
	if record["route"] != "/trusted" {
		t.Fatalf("route = %v, want /trusted", record["route"])
	}
}

func TestRecoveryDoesNotLogPanicText(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	logger := logging.New(logging.Config{Level: slog.LevelInfo, Format: "json"}, &output)

	router := gin.New()
	router.Use(RequestLogging(Config{Logger: logger}))
	router.Use(Recovery(logger))
	router.GET("/panic", func(_ *gin.Context) {
		panic("secret request body")
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	if bytes.Contains(output.Bytes(), []byte("secret request body")) {
		t.Fatalf("panic text leaked into logs: %s", output.String())
	}

	lines := bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("log lines = %d, want recovery and completion: %s", len(lines), output.String())
	}
	var completion map[string]any
	if err := json.Unmarshal(lines[1], &completion); err != nil {
		t.Fatalf("decode completion log: %v", err)
	}
	if completion["msg"] != "http.request.completed" ||
		completion["status"] != float64(http.StatusInternalServerError) {
		t.Fatalf("completion record = %#v", completion)
	}
}
