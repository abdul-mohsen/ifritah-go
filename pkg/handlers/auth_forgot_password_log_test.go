package handlers

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"

	db "ifritah/web-service-gin/pkg/db/gen"
)

// The reset token is 64 random bytes, hex-encoded: 128 hex characters.
var resetTokenPattern = regexp.MustCompile(`[0-9a-f]{128}`)

// A reset token is a credential. It must not reach stdout or the log.
func TestForgotPasswordDoesNotPrintResetToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()
	mock.ExpectQuery("SELECT id FROM user WHERE email").
		WithArgs("user@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	mock.ExpectExec("INSERT INTO password_reset_tokens").
		WillReturnResult(sqlmock.NewResult(1, 1))

	h := New(sqlDB, db.New(sqlDB), nil)

	var logged bytes.Buffer
	defer func(prev io.Writer) { log.SetOutput(prev) }(log.Writer())
	log.SetOutput(&logged)

	printed := captureStdout(t, func() {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v2/forgot-password", strings.NewReader(`{"email":"user@example.com"}`))
		c.Request.Header.Set("Content-Type", "application/json")
		h.ForgotPassword(c)
		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
	if resetTokenPattern.MatchString(printed) || resetTokenPattern.MatchString(logged.String()) {
		t.Fatal("reset token was written to stdout or the log")
	}
}

// captureStdout returns everything f writes to os.Stdout.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	stdout := os.Stdout
	os.Stdout = w
	f()
	os.Stdout = stdout
	if err := w.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return string(out)
}
