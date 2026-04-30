package handlers

// User management endpoints.
// All endpoints in this file are admin-only and are wired in main.go behind
// handlers.RequireAdmin(). The Login/Register/GetMe handlers live in auth.go.
//
// Routes:
//   GET    /api/v2/users/me           → GetMe                    (any auth)
//   GET    /api/v2/user/all           → ListUsers                (admin)
//   GET    /api/v2/user/:id           → GetUserByID              (admin)
//   POST   /api/v2/user               → CreateUser               (admin, sets role)
//   PUT    /api/v2/user/:id           → UpdateUser               (admin)
//   DELETE /api/v2/user/:id           → DeleteUser               (admin, soft via is_active)
//   POST   /api/v2/user/:id/password  → AdminResetUserPassword   (admin)

import (
	"database/sql"
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

// ── ListUsers ──────────────────────────────────────────────────────────────

func (h *handler) ListUsers(c *gin.Context) {
	rows, err := h.DB.Query(`
		SELECT id, username, COALESCE(full_name,''), COALESCE(email,''), COALESCE(phone,''),
		       role, is_active, company_id,
		       DATE_FORMAT(last_login, '%Y-%m-%dT%H:%i:%sZ')
		FROM user
		ORDER BY id ASC
	`)
	if err != nil {
		log.Printf("ListUsers query: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to list users"})
		return
	}
	defer rows.Close()

	var users []userResponse
	for rows.Next() {
		var u userResponse
		var companyID sql.NullInt64
		var lastLogin sql.NullString
		if err := rows.Scan(&u.ID, &u.Username, &u.FullName, &u.Email, &u.Phone,
			&u.Role, &u.IsActive, &companyID, &lastLogin); err != nil {
			log.Printf("ListUsers scan: %v", err)
			continue
		}
		if companyID.Valid {
			u.CompanyID = &companyID.Int64
		}
		if lastLogin.Valid {
			s := lastLogin.String
			u.LastLogin = &s
		}
		users = append(users, u)
	}
	c.JSON(http.StatusOK, gin.H{"data": users})
}

// ── GetUserByID ────────────────────────────────────────────────────────────

func (h *handler) GetUserByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid user id"})
		return
	}

	var u userResponse
	var companyID sql.NullInt64
	var lastLogin sql.NullString
	err = h.DB.QueryRow(`
		SELECT id, username, COALESCE(full_name,''), COALESCE(email,''), COALESCE(phone,''),
		       role, is_active, company_id,
		       DATE_FORMAT(last_login, '%Y-%m-%dT%H:%i:%sZ')
		FROM user WHERE id = ?
	`, id).Scan(&u.ID, &u.Username, &u.FullName, &u.Email, &u.Phone,
		&u.Role, &u.IsActive, &companyID, &lastLogin)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"detail": "user not found"})
		return
	}
	if err != nil {
		log.Printf("GetUserByID: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to fetch user"})
		return
	}
	if companyID.Valid {
		u.CompanyID = &companyID.Int64
	}
	if lastLogin.Valid {
		s := lastLogin.String
		u.LastLogin = &s
	}
	c.JSON(http.StatusOK, gin.H{"data": u})
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
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request: " + err.Error()})
		return
	}
	if !allowedRoles[req.Role] {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid role; expected admin, manager or employee"})
		return
	}

	// Default company to the caller's company if not provided.
	if req.CompanyID == nil {
		var c64 sql.NullInt64
		_ = h.DB.QueryRow("SELECT company_id FROM user WHERE id = ?", GetSessionInfo(c).id).Scan(&c64)
		if c64.Valid {
			req.CompanyID = &c64.Int64
		}
	}

	// Username uniqueness
	var exists int
	_ = h.DB.QueryRow("SELECT COUNT(*) FROM user WHERE username = ?", req.Username).Scan(&exists)
	if exists > 0 {
		c.JSON(http.StatusConflict, gin.H{"detail": "username already exists"})
		return
	}
	if req.Email != "" {
		_ = h.DB.QueryRow("SELECT COUNT(*) FROM user WHERE email = ?", req.Email).Scan(&exists)
		if exists > 0 {
			c.JSON(http.StatusConflict, gin.H{"detail": "email already exists"})
			return
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("CreateUser hash: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to hash password"})
		return
	}

	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}

	res, err := h.DB.Exec(`
		INSERT INTO user (username, full_name, password, email, phone, company_id, is_active, role)
		VALUES (?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?)
	`, req.Username, req.FullName, string(hash), req.Email, req.Phone, req.CompanyID, active, req.Role)
	if err != nil {
		log.Printf("CreateUser insert: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to create user"})
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
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid user id"})
		return
	}

	var req adminUpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request: " + err.Error()})
		return
	}
	if req.Role != nil && !allowedRoles[*req.Role] {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid role"})
		return
	}

	// Prevent the only admin from being demoted or deactivated by mistake.
	if (req.Role != nil && *req.Role != RoleAdmin) || (req.IsActive != nil && !*req.IsActive) {
		var currentRole string
		_ = h.DB.QueryRow("SELECT role FROM user WHERE id = ?", id).Scan(&currentRole)
		if currentRole == RoleAdmin {
			var adminCount int
			_ = h.DB.QueryRow("SELECT COUNT(*) FROM user WHERE role = 'admin' AND is_active = 1").Scan(&adminCount)
			if adminCount <= 1 {
				c.JSON(http.StatusBadRequest, gin.H{"detail": "cannot demote or deactivate the last admin"})
				return
			}
		}
	}

	// Build dynamic UPDATE.
	sets := []string{}
	args := []any{}
	if req.FullName != nil {
		sets = append(sets, "full_name = ?")
		args = append(args, *req.FullName)
	}
	if req.Email != nil {
		sets = append(sets, "email = NULLIF(?, '')")
		args = append(args, *req.Email)
	}
	if req.Phone != nil {
		sets = append(sets, "phone = NULLIF(?, '')")
		args = append(args, *req.Phone)
	}
	if req.Role != nil {
		sets = append(sets, "role = ?")
		args = append(args, *req.Role)
	}
	if req.IsActive != nil {
		sets = append(sets, "is_active = ?")
		args = append(args, *req.IsActive)
	}
	if req.CompanyID != nil {
		sets = append(sets, "company_id = ?")
		args = append(args, *req.CompanyID)
	}
	if len(sets) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "no fields to update"})
		return
	}

	q := "UPDATE user SET " + joinStrings(sets, ", ") + " WHERE id = ?"
	args = append(args, id)
	res, err := h.DB.Exec(q, args...)
	if err != nil {
		log.Printf("UpdateUser: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to update user"})
		return
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"detail": "updated"})
}

// ── DeleteUser ─────────────────────────────────────────────────────────────

func (h *handler) DeleteUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid user id"})
		return
	}

	caller := GetSessionInfo(c).id
	if caller == id {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "cannot deactivate your own account"})
		return
	}

	// Refuse to deactivate the last active admin.
	var role string
	if err := h.DB.QueryRow("SELECT role FROM user WHERE id = ?", id).Scan(&role); err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"detail": "user not found"})
		return
	} else if err != nil {
		log.Printf("DeleteUser lookup: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to delete user"})
		return
	}
	if role == RoleAdmin {
		var adminCount int
		_ = h.DB.QueryRow("SELECT COUNT(*) FROM user WHERE role = 'admin' AND is_active = 1").Scan(&adminCount)
		if adminCount <= 1 {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "cannot deactivate the last admin"})
			return
		}
	}

	if _, err := h.DB.Exec("UPDATE user SET is_active = 0 WHERE id = ?", id); err != nil {
		log.Printf("DeleteUser: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to delete user"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"detail": "user deactivated"})
}

// ── AdminResetUserPassword ─────────────────────────────────────────────────

type adminResetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

func (h *handler) AdminResetUserPassword(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid user id"})
		return
	}
	var req adminResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request: " + err.Error()})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("AdminResetUserPassword hash: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to hash password"})
		return
	}
	res, err := h.DB.Exec("UPDATE user SET password = ? WHERE id = ?", string(hash), id)
	if err != nil {
		log.Printf("AdminResetUserPassword: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to reset password"})
		return
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "user not found"})
		return
	}

	// Invalidate all refresh tokens for this user so they have to log in again.
	_, _ = h.DB.Exec("DELETE FROM refresh_token WHERE user_id = ?", id)

	c.JSON(http.StatusOK, gin.H{"detail": "password reset"})
}

// joinStrings is a tiny helper that avoids pulling in strings.Join just for the
// dynamic UPDATE above.
func joinStrings(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}
