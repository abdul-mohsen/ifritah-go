package handlers

// User management endpoints.
// All endpoints in this file are admin-only and are wired in main.go behind
// handlers.RequireAdmin(). The Login/Register/GetMe handlers live in auth.go.
//
// Routes:
//   GET    /api/v2/users/me           → GetMe                    (any auth)
//   POST   /api/v2/user/all           → ListUsers                (admin) — keyset envelope
//   GET    /api/v2/user/:id           → GetUserByID              (admin)
//   POST   /api/v2/user               → CreateUser               (admin, sets role)
//   PUT    /api/v2/user/:id           → UpdateUser               (admin)
//   DELETE /api/v2/user/:id           → DeleteUser               (admin, soft via is_active)
//   POST   /api/v2/user/:id/password  → AdminResetUserPassword   (admin)

import (
	"database/sql"
	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"
	"ifritah/web-service-gin/pkg/pagination"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ── Helpers ────────────────────────────────────────────────────────────────

var allowedRoles = map[string]bool{
	RoleAdmin:    true,
	RoleManager:  true,
	RoleEmployee: true,
}

type userResponse struct {
	ID        int64   `json:"id"`
	Username  string  `json:"username"`
	FullName  string  `json:"full_name"`
	Email     string  `json:"email"`
	Phone     string  `json:"phone"`
	Role      string  `json:"role"`
	IsActive  bool    `json:"is_active"`
	CompanyID *int64  `json:"company_id"`
	LastLogin *string `json:"last_login"`
}

// fromListRow / fromAdminRow convert sqlc-generated row types to userResponse.
// LastLogin is `interface{}` on ListUsersRow because the new keyset query
// wraps DATE_FORMAT in CAST(COALESCE(..., ”) AS CHAR(40)) — sqlc can't
// statically prove the result is non-null without extra hints, so we
// runtime-coerce here. NULL becomes "" which buildUserResponse maps back to
// nil on the JSON side, preserving the wire shape.
func fromListRow(r db.ListUsersRow) userResponse {
	ll, _ := r.LastLogin.(string)
	if b, ok := r.LastLogin.([]byte); ok {
		ll = string(b)
	}
	return buildUserResponse(int64(r.ID), r.Username, r.FullName, r.Email, r.Phone, string(r.Role), r.IsActive, r.CompanyID, ll)
}

func fromAdminRow(r db.GetUserAdminRow) userResponse {
	return buildUserResponse(int64(r.ID), r.Username, r.FullName, r.Email, r.Phone, string(r.Role), r.IsActive, r.CompanyID, r.LastLogin)
}

func buildUserResponse(id int64, username, fullName, email, phone, role string, isActive bool, companyID *int32, lastLogin string) userResponse {
	u := userResponse{
		ID:       id,
		Username: username,
		FullName: fullName,
		Email:    email,
		Phone:    phone,
		Role:     role,
		IsActive: isActive,
	}
	if companyID != nil {
		cid := int64(*companyID)
		u.CompanyID = &cid
	}
	if lastLogin != "" {
		ll := lastLogin
		u.LastLogin = &ll
	}
	return u
}

// Keyset-paginated user list. Sort: id ASC.
const userListSort = "id"

func (h *handler) ListUsers(c *gin.Context) {
	var req model.PaginationRequest
	c.ShouldBindJSON(&req)

	listReq := pagination.ListRequest{
		Limit:      int(req.Limit),
		Cursor:     req.Cursor,
		Sort:       req.Sort,
		Dir:        req.Dir,
		Query:      req.Query,
		PageNumber: int(req.PageNumber),
		PageSize:   int(req.PageSize),
	}
	if err := listReq.Validate(userListSort); err != nil {
		log.Printf("ListUsers: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}

	cur, _ := listReq.DecodedCursor()
	cursorID := cursorIDOnly(cur)
	limit := listReq.EffectiveLimit()

	queryLike, _ := buildLikeAndDigitsExact(req.Query)
	phonePrefix := buildPlainPrefixFilter(req.Phone)
	emailPrefix := buildPlainPrefixFilter(req.Email)

	var cursorIDPtr *int32
	if cursorID != nil {
		v := int32(*cursorID)
		cursorIDPtr = &v
	}

	params := db.ListUsersParams{
		QueryLike:   queryLike,
		PhonePrefix: phonePrefix,
		EmailPrefix: emailPrefix,
		CursorID:    cursorIDPtr,
		Limit:       int32(limit + 1),
	}

	rows, err := h.queries.ListUsers(c.Request.Context(), params)
	if err != nil {
		log.Printf("ListUsers query: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": ErrListUsers})
		return
	}

	users := make([]userResponse, 0, len(rows))
	for _, r := range rows {
		users = append(users, fromListRow(r))
	}

	envelope := pagination.BuildEnvelope(
		users,
		limit,
		userListSort,
		func(u userResponse) []any { return []any{u.ID} },
	)
	c.JSON(http.StatusOK, envelope)
}

// ── GetUserByID ────────────────────────────────────────────────────────────

func (h *handler) GetUserByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": ErrInvalidUserID})
		return
	}
	row, err := h.queries.GetUserAdmin(c.Request.Context(), int32(id))
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"detail": ErrUserNotFound})
		return
	}
	if err != nil {
		log.Printf("GetUserByID: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": ErrFetchUser})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": fromAdminRow(row)})
}

// ── CreateUser ─────────────────────────────────────────────────────────────

type adminCreateUserRequest struct {
	Username  string `json:"username"   binding:"required,min=3,max=45"`
	Password  string `json:"password"   binding:"required,min=6"`
	Role      string `json:"role"       binding:"required"`
	FullName  string `json:"full_name"`
	Email     string `json:"email"      binding:"omitempty,email"`
	Phone     string `json:"phone"`
	CompanyID *int64 `json:"company_id"`
	IsActive  *bool  `json:"is_active"`
}

func (h *handler) CreateUser(c *gin.Context) {
	var req adminCreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": invalidRequestDetail(err)})
		return
	}
	if !allowedRoles[req.Role] {
		c.JSON(http.StatusBadRequest, gin.H{"detail": ErrInvalidRole + "; expected admin, manager or employee"})
		return
	}
	ctx := c.Request.Context()

	// Default company to the caller's company if not provided.
	var companyID *int32
	if req.CompanyID != nil {
		cid := int32(*req.CompanyID)
		companyID = &cid
	} else {
		cid, err := h.queries.GetUserCompanyID(ctx, int32(GetSessionInfo(c).id))
		if err != nil && err != sql.ErrNoRows {
			log.Printf("CreateUser: lookup caller company_id: %v", err)
		}
		companyID = cid
	}

	// Username uniqueness
	nameCount, err := h.queries.CountUsersByUsername(ctx, req.Username)
	if err != nil {
		log.Printf("CreateUser: username uniqueness check: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": ErrDatabase})
		return
	}
	if nameCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"detail": ErrUsernameExists})
		return
	}
	if req.Email != "" {
		email := req.Email
		emailCount, err := h.queries.CountUsersByEmail(ctx, &email)
		if err != nil {
			log.Printf("CreateUser: email uniqueness check: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"detail": ErrDatabase})
			return
		}
		if emailCount > 0 {
			c.JSON(http.StatusConflict, gin.H{"detail": ErrEmailExists})
			return
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("CreateUser hash: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": ErrHashPassword})
		return
	}

	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}

	fullName := req.FullName
	res, err := h.queries.CreateUserAdmin(ctx, db.CreateUserAdminParams{
		Username:  req.Username,
		FullName:  &fullName,
		Password:  string(hash),
		Email:     req.Email,
		Phone:     req.Phone,
		CompanyID: companyID,
		IsActive:  active,
		Role:      db.UserRole(req.Role),
	})
	if err != nil {
		log.Printf("CreateUser insert: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": ErrCreateUser})
		return
	}
	id, _ := res.LastInsertId()

	c.JSON(http.StatusCreated, gin.H{
		"id":         id,
		"username":   req.Username,
		"role":       req.Role,
		"is_active":  active,
		"company_id": req.CompanyID,
	})
}

// ── UpdateUser ─────────────────────────────────────────────────────────────

type adminUpdateUserRequest struct {
	FullName  *string `json:"full_name"`
	Email     *string `json:"email"      binding:"omitempty,email"`
	Phone     *string `json:"phone"`
	Role      *string `json:"role"`
	IsActive  *bool   `json:"is_active"`
	CompanyID *int64  `json:"company_id"`
}

func (h *handler) UpdateUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": ErrInvalidUserID})
		return
	}

	var req adminUpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": invalidRequestDetail(err)})
		return
	}
	if req.Role != nil && !allowedRoles[*req.Role] {
		c.JSON(http.StatusBadRequest, gin.H{"detail": ErrInvalidRole})
		return
	}
	ctx := c.Request.Context()

	// Prevent the only admin from being demoted or deactivated by mistake.
	if (req.Role != nil && *req.Role != RoleAdmin) || (req.IsActive != nil && !*req.IsActive) {
		currentRole, err := h.queries.GetUserRole(ctx, int32(id))
		if err != nil && err != sql.ErrNoRows {
			log.Printf("UpdateUser: lookup current role: %v", err)
		}
		if string(currentRole) == RoleAdmin {
			adminCount, err := h.queries.CountActiveAdmins(ctx)
			if err != nil {
				log.Printf("UpdateUser: count admins: %v", err)
			}
			if adminCount <= 1 {
				c.JSON(http.StatusBadRequest, gin.H{"detail": "cannot demote or deactivate the last admin"})
				return
			}
		}
	}

	// Build the update params: nil pointer / null wrapper ⇒ COALESCE keeps
	// the existing column value. Empty-string email/phone is normalised to
	// NULL to match the previous NULLIF behaviour.
	params := db.UpdateUserAdminParams{ID: int32(id)}
	hasUpdate := false
	if req.FullName != nil {
		params.FullName = req.FullName
		hasUpdate = true
	}
	if req.Email != nil {
		if *req.Email != "" {
			params.Email = req.Email
		}
		hasUpdate = true
	}
	if req.Phone != nil {
		if *req.Phone != "" {
			params.Phone = req.Phone
		}
		hasUpdate = true
	}
	if req.Role != nil {
		params.Role = db.NullUserRole{UserRole: db.UserRole(*req.Role), Valid: true}
		hasUpdate = true
	}
	if req.IsActive != nil {
		params.IsActive = sql.NullBool{Bool: *req.IsActive, Valid: true}
		hasUpdate = true
	}
	if req.CompanyID != nil {
		cid := int32(*req.CompanyID)
		params.CompanyID = &cid
		hasUpdate = true
	}
	if !hasUpdate {
		c.JSON(http.StatusBadRequest, gin.H{"detail": ErrNoFieldsToUpdate})
		return
	}

	res, err := h.queries.UpdateUserAdmin(ctx, params)
	if err != nil {
		log.Printf("UpdateUser: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": ErrUpdateUser})
		return
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": ErrUserNotFound})
		return
	}
	c.JSON(http.StatusOK, gin.H{"detail": "updated"})
}

// ── DeleteUser ─────────────────────────────────────────────────────────────

func (h *handler) DeleteUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": ErrInvalidUserID})
		return
	}

	caller := GetSessionInfo(c).id
	if caller == id {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "cannot deactivate your own account"})
		return
	}
	ctx := c.Request.Context()

	// Refuse to deactivate the last active admin.
	role, err := h.queries.GetUserRole(ctx, int32(id))
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"detail": ErrUserNotFound})
		return
	} else if err != nil {
		log.Printf("DeleteUser lookup: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": ErrDeleteUser})
		return
	}
	if string(role) == RoleAdmin {
		adminCount, err := h.queries.CountActiveAdmins(ctx)
		if err != nil {
			log.Printf("DeleteUser: count admins: %v", err)
		}
		if adminCount <= 1 {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "cannot deactivate the last admin"})
			return
		}
	}

	if err := h.queries.DeactivateUser(ctx, int32(id)); err != nil {
		log.Printf("DeleteUser: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": ErrDeleteUser})
		return
	}
	c.JSON(http.StatusOK, gin.H{"detail": "user deactivated"})
}

// ── AdminResetUserPassword ─────────────────────────────────────────────────

type adminResetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

func (h *handler) AdminResetUserPassword(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": ErrInvalidUserID})
		return
	}
	var req adminResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": invalidRequestDetail(err)})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("AdminResetUserPassword hash: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": ErrHashPassword})
		return
	}
	ctx := c.Request.Context()
	res, err := h.queries.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{
		Password: string(hash),
		ID:       int32(id),
	})
	if err != nil {
		log.Printf("AdminResetUserPassword: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": ErrResetPassword})
		return
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": ErrUserNotFound})
		return
	}

	// Invalidate all refresh tokens for this user so they have to log in again.
	if err := h.queries.DeleteRefreshTokensForUser(ctx, int32(id)); err != nil {
		log.Printf("AdminResetUserPassword: revoke refresh tokens: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"detail": "password reset"})
}
