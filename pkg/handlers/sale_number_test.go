package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

// allocateForBranch takes the next bill number of branch 7, as a sale does.
func allocateForBranch(t *testing.T, h *handler) (uint64, error) {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/bill", nil)
	return allocateBillNumber(h.queries, c, 7)
}

// The bill counter has no fallback. An error and a zero are failed sales, never numbers.
func TestAllocateBillNumberHasNoFallback(t *testing.T) {
	cases := []struct {
		name    string
		expect  func(mock sqlmock.Sqlmock)
		want    uint64
		wantErr bool
	}{
		{
			name: "an error is returned, not replaced",
			expect: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO branch_sequence").WillReturnError(errors.New("lock wait timeout"))
			},
			wantErr: true,
		},
		{
			name:    "a zero is refused, not used as a number",
			expect:  func(mock sqlmock.Sqlmock) { expectNumberAllocation(mock, 0) },
			wantErr: true,
		},
		{
			name:   "the first number of a branch is 1",
			expect: func(mock sqlmock.Sqlmock) { expectNumberAllocation(mock, 1) },
			want:   1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, mock := newSaleTestHandler(t)
			tc.expect(mock)
			number, err := allocateForBranch(t, h)
			if (err != nil) != tc.wantErr || number != tc.want {
				t.Fatalf("allocateBillNumber = %d, %v; want %d, error %t", number, err, tc.want, tc.wantErr)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
