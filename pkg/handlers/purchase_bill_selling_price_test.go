package handlers

import (
	"database/sql"
	"net/http/httptest"
	"testing"

	"ifritah/web-service-gin/pkg/model"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

func newRoleContext(role string) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", nil)
	if role != "" {
		c.Set("user_role", role)
	}
	return c
}

func TestCanOverrideSellingPrice(t *testing.T) {
	cases := []struct {
		role string
		want bool
	}{
		{RoleAdmin, true},
		{RoleManager, true},
		{RoleEmployee, false},
		{"", false},
	}
	for _, tc := range cases {
		if got := canOverrideSellingPrice(newRoleContext(tc.role)); got != tc.want {
			t.Errorf("canOverrideSellingPrice(role=%q) = %v, want %v", tc.role, got, tc.want)
		}
	}
}

func TestGetDefaultMarkupPercentage(t *testing.T) {
	cases := []struct {
		name    string
		row     string
		queryOK bool
		want    decimal.Decimal
	}{
		{"valid percentage", "20", true, decimal.NewFromInt(20)},
		{"blank value falls back to zero", "", true, decimal.Zero},
		{"garbage value falls back to zero", "not-a-number", true, decimal.Zero},
		{"negative value falls back to zero", "-5", true, decimal.Zero},
		{"missing row falls back to zero", "", false, decimal.Zero},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, mock, cleanup := newPurchaseBillTestHandler(t)
			defer cleanup()

			q := mock.ExpectQuery("SELECT COALESCE\\(value, ''\\) AS value").WithArgs("default_markup_percentage")
			if tc.queryOK {
				q.WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(tc.row))
			} else {
				q.WillReturnError(sql.ErrNoRows)
			}

			got := h.getDefaultMarkupPercentage(newRoleContext(RoleAdmin))
			if !got.Equal(tc.want) {
				t.Fatalf("getDefaultMarkupPercentage() = %s, want %s", got, tc.want)
			}
			assertMockExpectations(t, mock)
		})
	}
}

func TestResolveNewProductSellingPrice(t *testing.T) {
	explicit := decimal.NewFromInt(150)

	t.Run("admin explicit selling price wins", func(t *testing.T) {
		h, mock, cleanup := newPurchaseBillTestHandler(t)
		defer cleanup()
		// No settings lookup expected: an explicit override short-circuits it.
		product := model.PurchaseBillProduct{CostPrice: decimal.NewFromInt(100), SellingPrice: &explicit}

		got := h.resolveNewProductSellingPrice(newRoleContext(RoleAdmin), product, true)
		if !got.Equal(explicit) {
			t.Fatalf("got %s, want %s", got, explicit)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("admin without explicit price falls back to markup", func(t *testing.T) {
		h, mock, cleanup := newPurchaseBillTestHandler(t)
		defer cleanup()
		mock.ExpectQuery("SELECT COALESCE\\(value, ''\\) AS value").
			WithArgs("default_markup_percentage").
			WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("20"))
		product := model.PurchaseBillProduct{CostPrice: decimal.NewFromInt(100)}

		got := h.resolveNewProductSellingPrice(newRoleContext(RoleAdmin), product, true)
		want := decimal.NewFromInt(120)
		if !got.Equal(want) {
			t.Fatalf("got %s, want %s", got, want)
		}
		assertMockExpectations(t, mock)
	})

	t.Run("employee-submitted selling price is ignored, markup applies", func(t *testing.T) {
		h, mock, cleanup := newPurchaseBillTestHandler(t)
		defer cleanup()
		mock.ExpectQuery("SELECT COALESCE\\(value, ''\\) AS value").
			WithArgs("default_markup_percentage").
			WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("30"))
		// Even if the request body somehow carries a selling_price, a
		// non-admin/manager caller must never have it applied.
		product := model.PurchaseBillProduct{CostPrice: decimal.NewFromInt(100), SellingPrice: &explicit}

		got := h.resolveNewProductSellingPrice(newRoleContext(RoleEmployee), product, false)
		want := decimal.NewFromInt(130)
		if !got.Equal(want) {
			t.Fatalf("got %s, want %s", got, want)
		}
		assertMockExpectations(t, mock)
	})
}

func TestUpdatePurchaseInventoryProductPriceOverride(t *testing.T) {
	productCols := []string{"id", "article_id", "store_id", "status", "shelf_number", "min_stock",
		"cost_price", "price", "quantity", "is_deleted", "name"}

	newProductID := int32(5)
	newPrice := decimal.NewFromInt(199)

	cases := []struct {
		name               string
		role               string
		canOverridePrice   bool
		wantPersistedPrice string
	}{
		{"admin/manager override updates catalog price", RoleAdmin, true, "199"},
		{"employee cannot override catalog price", RoleEmployee, false, "80"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, mock, cleanup := newPurchaseBillTestHandler(t)
			defer cleanup()
			mock.ExpectQuery("select .* from product p where p\\.id = \\? and p\\.is_deleted").
				WithArgs(uint64(5)).
				WillReturnRows(sqlmock.NewRows(productCols).
					AddRow(5, nil, int32(1), 0, "A1", 5, "50.00", "80.00", "10.000", false, "Widget"))
			mock.ExpectExec("update product").
				WithArgs(tc.wantPersistedPrice, "40", "A2", sqlmock.AnyArg(), uint64(5)).
				WillReturnResult(sqlmock.NewResult(0, 1))

			product := model.PurchaseBillProduct{
				ProductId: &newProductID, CostPrice: decimal.NewFromFloat(40), Quantity: decimal.NewFromInt(2),
				ShelfNumber: strPtr("A2"), SellingPrice: &newPrice,
			}
			id, err := updatePurchaseInventoryProduct(h.queries, newRoleContext(tc.role), product, 1, true, tc.canOverridePrice)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id == nil || *id != 5 {
				t.Fatalf("productID = %v, want 5", id)
			}
			assertMockExpectations(t, mock)
		})
	}
}
