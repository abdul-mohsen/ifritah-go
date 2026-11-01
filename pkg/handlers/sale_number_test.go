package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

// The bill counter has no fallback. An error and a zero are failed sales, never numbers.
func TestAllocateBillNumberHasNoFallback(t *testing.T) {
	allocate := func(t *testing.T, mock sqlmock.Sqlmock, h *handler) (uint64, error) {
		t.Helper()
		_ = mock
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/bill", nil)
		return allocateBillNumber(h.queries, c, 7)
	}

	t.Run("an error is returned, not replaced", func(t *testing.T) {
		h, mock := newSaleTestHandler(t)
		mock.ExpectExec("INSERT INTO branch_sequence").
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnError(errors.New("lock wait timeout"))
		number, err := allocate(t, mock, h)
		if err == nil || number != 0 {
			t.Fatalf("allocateBillNumber = %d, %v; want an error and no number", number, err)
		}
	})

	t.Run("a zero is refused, not used as a number", func(t *testing.T) {
		h, mock := newSaleTestHandler(t)
		mock.ExpectExec("INSERT INTO branch_sequence").
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))
		number, err := allocate(t, mock, h)
		if err == nil || number != 0 {
			t.Fatalf("allocateBillNumber = %d, %v; want an error and no number", number, err)
		}
	})

	t.Run("the first number of a branch is 1", func(t *testing.T) {
		h, mock := newSaleTestHandler(t)
		mock.ExpectExec("INSERT INTO branch_sequence").
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))
		number, err := allocate(t, mock, h)
		if err != nil || number != 1 {
			t.Fatalf("allocateBillNumber = %d, %v; want 1", number, err)
		}
	})
}
