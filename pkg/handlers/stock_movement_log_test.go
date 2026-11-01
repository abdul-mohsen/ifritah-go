package handlers

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	db "ifritah/web-service-gin/pkg/db/gen"
)

// A stock movement carries the user's reason and note. Neither may reach the log.
func TestInsertStockMovementDoesNotLogPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()
	mock.ExpectExec("INSERT INTO stock_movements").WillReturnResult(sqlmock.NewResult(1, 1))

	reason := "reason-marker-9c1e"
	note := "note-marker-7f3a"

	var logged bytes.Buffer
	defer func(prev io.Writer) { log.SetOutput(prev) }(log.Writer())
	log.SetOutput(&logged)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	err = insertStockMovement(db.New(sqlDB), c, 1, 1, decimal.NewFromInt(2), "sale", "bill",
		nil, nil, &reason, &note, nil, time.Now())
	if err != nil {
		t.Fatalf("insertStockMovement: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
	if strings.Contains(logged.String(), reason) || strings.Contains(logged.String(), note) {
		t.Fatal("stock movement reason or note was written to the log")
	}
}
