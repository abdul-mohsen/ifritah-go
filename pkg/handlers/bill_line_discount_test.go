package handlers

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"
)

func TestAddProductToBillPersistsLineDiscount(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	name := "Brake pad"
	partName := "OEM-24"
	productID := uint64(7)
	price := decimal.NewFromInt(10)
	quantity := decimal.NewFromInt(2)
	discount := decimal.NewFromFloat(3.25)

	mock.ExpectExec(regexp.QuoteMeta(
		"insert into bill_product (name, product_id, price, quantity, bill_id, part_name, discount) values (?, ?, ?, ?, ?, ?, ?)",
	)).
		WithArgs(name, productID, price, quantity, uint64(42), partName, discount).
		WillReturnResult(sqlmock.NewResult(1, 1))

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	err = addProductToBill(db.New(sqlDB), c, []model.BillProduct{{
		Name: name, PartName: partName, ProductId: &productID,
		Price: price, Quantity: quantity, Discount: discount,
	}}, 42)
	if err != nil {
		t.Fatalf("addProductToBill: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
