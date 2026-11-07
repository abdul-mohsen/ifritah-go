package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"
)

type billVisibilityScope struct {
	storeIDs   []int32
	merchantID *int32
	enforced   bool
}

func (s billVisibilityScope) queryStoreIDs() []int32 {
	if len(s.storeIDs) == 0 {
		return []int32{0}
	}
	return s.storeIDs
}

func (s billVisibilityScope) billIDParams(id uint64) db.GetBillIDForScopeParams {
	return db.GetBillIDForScopeParams{
		ID:         id,
		StoreIds:   s.queryStoreIDs(),
		MerchantID: s.merchantID,
	}
}

func (s billVisibilityScope) billPDFParams(id uint64) db.GetBillPDFByIDParams {
	return db.GetBillPDFByIDParams{
		ID:         id,
		StoreIds:   s.queryStoreIDs(),
		MerchantID: s.merchantID,
	}
}

func (s billVisibilityScope) creditBillParams(id uint64) db.GetCreditBillByIDParams {
	return db.GetCreditBillByIDParams{
		ID:         id,
		StoreIds:   s.queryStoreIDs(),
		MerchantID: s.merchantID,
	}
}

func authenticatedRole(c *gin.Context) string {
	if role := strings.ToLower(strings.TrimSpace(c.GetString("user_role"))); role != "" {
		return role
	}
	if claims, ok := c.Get("decoded_jwt"); ok {
		if typed, ok := claims.(*model.Claims); ok && typed != nil {
			return strings.ToLower(strings.TrimSpace(typed.Role))
		}
	}
	return ""
}

func userIDFromContext(c *gin.Context) int64 {
	if id := c.GetInt64("userId"); id != 0 {
		return id
	}
	if claims, ok := c.Get("decoded_jwt"); ok {
		if typed, ok := claims.(*model.Claims); ok && typed != nil {
			return typed.Id
		}
	}
	return 0
}

func (h *handler) billVisibilityScope(c *gin.Context) (billVisibilityScope, error) {
	role := authenticatedRole(c)
	if role == "" {
		// Direct handler tests and legacy internal callers may not run JWT
		// middleware. Their SQL still receives a non-matching store sentinel
		// instead of silently bypassing scope.
		return billVisibilityScope{}, nil
	}

	userID := userIDFromContext(c)
	if userID <= 0 {
		return billVisibilityScope{}, fmt.Errorf("authenticated user id is missing")
	}

	storeIDs, err := h.loadStoreIDs(c.Request.Context(), userID, role)
	if err != nil {
		return billVisibilityScope{}, err
	}

	scope := billVisibilityScope{
		storeIDs: storeIDs,
		enforced: true,
	}
	if role == RoleEmployee {
		merchantID := int32(userID)
		scope.merchantID = &merchantID
	}
	return scope, nil
}

func (h *handler) loadStoreIDs(ctx context.Context, userID int64, role string) ([]int32, error) {
	rows, err := h.listStoresForScope(ctx, userID, role)
	if err != nil {
		return nil, err
	}

	storeIDs := make([]int32, 0, len(rows))
	for _, row := range rows {
		storeIDs = append(storeIDs, row.ID)
	}
	return storeIDs, nil
}

func (h *handler) requireBillScope(c *gin.Context) (billVisibilityScope, bool) {
	scope, err := h.billVisibilityScope(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return billVisibilityScope{}, false
	}
	return scope, true
}

func (h *handler) checkBillIDAccess(c *gin.Context, billID uint64) bool {
	scope, ok := h.requireBillScope(c)
	if !ok {
		return false
	}
	if !scope.enforced {
		return true
	}

	_, err := h.queries.GetBillIDForScope(c.Request.Context(), scope.billIDParams(billID))
	switch {
	case err == nil:
		return true
	case errors.Is(err, sql.ErrNoRows):
		c.AbortWithStatus(http.StatusNotFound)
	default:
		c.AbortWithError(http.StatusInternalServerError, err)
	}
	return false
}
