package handlers

// User management API — admin/manager-driven CRUD.
//
// Backed by the existing `user` table (with `is_deleted` / `manager_id` added
// in migrations/20260430_02_user_management.sql) and the existing
// `user_permission` table (resource + can_view/can_add/can_edit/can_delete).
//
// Routes are registered in main.go under the `authorized` group; per-handler
// role checks (admin / manager) are enforced inline because there is no
// router-level role middleware in this codebase yet.

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	"ifritah/web-service-gin/pkg/model"
)

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

// sessionRole returns the role embedded in the JWT claims, or "" when missing.
func sessionRole(c *gin.Context) string {
	cs, ok := c.Get("decoded_jwt")
	if !ok {
		return ""
	}
	claims, ok := cs.(*model.Claims)
	if !ok || claims == nil {
		return ""
	}
	return claims.Role
}

// requireRole aborts with 403 unless the caller has one of the allowed roles.
func requireRole(c *gin.Context, allowed ...string) bool {
	role := sessionRole(c)
	for _, r := range allowed {
		if role == r {
			return true
		}
	}
	c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	return false
}

// validRoles are the only role values accepted on create/update.
var validRoles = map[string]struct{}{
	"admin":    {},
	"manager":  {},
	"employee": {},
}

// knownPermResources guards PUT /users/:id/permissions against typos.
// Mirrors the resources the frontend RBAC uses.
var knownPermResources = map[string]struct{}{
	"invoices":       {},
	"products":       {},
	"clients":        {},
	"suppliers":      {},
	"purchase_bills": {},
	"orders":         {},
	"branches":       {},
	"stores":         {},
	"cash_vouchers":  {},
	"stock":          {},
	"users":          {},
	"settings":       {},
	"reports":        {},
	"dashboard":      {},
}

// validActions are the action strings accepted in the permissions payload.
var validActions = map[string]struct{}{
	"view":   {},
	"add":    {},
	"edit":   {},
	"delete": {},
}

// userResponse is the JSON shape returned for a single user.
type userResponse struct {
	ID          int64   `json:"id"`
	Username    string  `json:"username"`
	Email       string  `json:"email"`
	FullName    string  `json:"full_name"`
	Phone       string  `json:"phone"`
	Role        string  `json:"role"`
	Active      bool    `json:"active"`
	ManagerID   *int64  `json:"manager_id"`
	CreatedAt   string  `json:"created_at"`
	LastLoginAt *string `json:"last_login_at"`
}

// scanUserRow scans a row of: id, username, email, full_name, phone, role,
// is_active, manager_id, created_at, last_login.
func scanUserRow(scanner interface {
	Scan(dest ...any) error
}) (userResponse, error) {
	var u userResponse
	var email, fullName, phone sql.NullString
	var managerID sql.NullInt64
	var lastLogin sql.NullString
	if err := scanner.Scan(
		&u.ID, &u.Username, &email, &fullName, &phone, &u.Role,
		&u.Active, &managerID, &u.CreatedAt, &lastLogin,
	); err != nil {
		return u, err
	}
	u.Email = email.String
	u.FullName = fullName.String
	u.Phone = phone.String
	if managerID.Valid {
		v := managerID.Int64
		u.ManagerID = &v
	}
	if lastLogin.Valid {
		v := lastLogin.String
		u.LastLoginAt = &v
	}
	return u, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// GET /api/v2/users
// ──────────────────────────────────────────────────────────────────────────────

// ListUsers returns a paginated user list filtered by `q` (username/email
// substring) and `role`. Soft-deleted users are excluded.
func (h *handler) ListUsers(c *gin.Context) {
	if !requireRole(c, "admin", "manager") {
		return
	}

	q := strings.TrimSpace(c.Query("q"))
	role := c.Query("role")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	if page < 0 {
		page = 0
	}
	per, _ := strconv.Atoi(c.DefaultQuery("per", "20"))
	if per <= 0 || per > 200 {
		per = 20
	}

	if role != "" {
		if _, ok := validRoles[role]; !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
			return
		}
	}
	like := "%" + q + "%"
	// Static SQL with sentinel filters: empty q/role means "no filter".
	const filterSQL = `is_deleted = 0
          AND (? = '' OR username LIKE ? OR email LIKE ? OR full_name LIKE ?)
          AND (? = '' OR role = ?)`

	var total int64
	if err := h.DB.QueryRow(
		`SELECT COUNT(*) FROM user WHERE `+filterSQL,
		q, like, like, like, role, role,
	).Scan(&total); err != nil {
		log.Printf("ListUsers count: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count users"})
		return
	}

	rows, err := h.DB.Query(
		`SELECT id, username, email, COALESCE(full_name,'') AS full_name,
                COALESCE(phone,'') AS phone, role, is_active, manager_id,
                created_at, last_login
         FROM user
         WHERE `+filterSQL+`
         ORDER BY id ASC
         LIMIT ? OFFSET ?`,
		q, like, like, like, role, role, per, page*per)
	if err != nil {
		log.Printf("ListUsers query: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list users"})
		return
	}
	defer rows.Close()

	items := make([]userResponse, 0, per)
	for rows.Next() {
		u, err := scanUserRow(rows)
		if err != nil {
			log.Printf("ListUsers scan: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to scan users"})
			return
		}
		items = append(items, u)
	}

	c.JSON(http.StatusOK, gin.H{
		"items":    items,
		"page":     page,
		"per_page": per,
		"total":    total,
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// GET /api/v2/users/:id
// ──────────────────────────────────────────────────────────────────────────────

// GetUserByID returns one user. Admin/manager can view anyone; an employee
// may only view themselves.
func (h *handler) GetUserByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	role := sessionRole(c)
	self := GetSessionInfo(c).id
	if role != "admin" && role != "manager" && id != self {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	row := h.DB.QueryRow(`
        SELECT id, username, email, COALESCE(full_name,'') AS full_name,
               COALESCE(phone,'') AS phone, role, is_active, manager_id,
               created_at, last_login
        FROM user WHERE id = ? AND is_deleted = 0`, id)
	u, err := scanUserRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if err != nil {
		log.Printf("GetUserByID: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user"})
		return
	}
	c.JSON(http.StatusOK, u)
}

// ──────────────────────────────────────────────────────────────────────────────
// POST /api/v2/users
// ──────────────────────────────────────────────────────────────────────────────

type createUserRequest struct {
	Username  string `json:"username"  binding:"required,min=3,max=45"`
	Email     string `json:"email"     binding:"omitempty,email"`
	FullName  string `json:"full_name"`
	Phone     string `json:"phone"`
	Password  string `json:"password"  binding:"required,min=6"`
	Role      string `json:"role"      binding:"required"`
	ManagerID *int64 `json:"manager_id"`
	Active    *bool  `json:"active"`
}

// CreateUser provisions a new user. Manager can only create employees;
// admin can create any role.
func (h *handler) CreateUser(c *gin.Context) {
	if !requireRole(c, "admin", "manager") {
		return
	}
	role := sessionRole(c)

	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, ok := validRoles[req.Role]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
		return
	}
	if role == "manager" && req.Role != "employee" {
		c.JSON(http.StatusForbidden, gin.H{"error": "managers may only create employees"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("CreateUser bcrypt: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	res, err := h.DB.Exec(`
        INSERT INTO user (username, email, full_name, phone, password, role, manager_id, is_active)
        VALUES (?,?,?,?,?,?,?,?)`,
		req.Username, nullableString(req.Email), nullableString(req.FullName),
		nullableString(req.Phone), string(hash), req.Role, req.ManagerID, boolToInt(active))
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			c.JSON(http.StatusConflict, gin.H{"error": "username_taken"})
			return
		}
		log.Printf("CreateUser insert: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}
	id, _ := res.LastInsertId()
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

// ──────────────────────────────────────────────────────────────────────────────
// PUT /api/v2/users/:id
// ──────────────────────────────────────────────────────────────────────────────

type updateUserRequest struct {
	Email     *string `json:"email"`
	FullName  *string `json:"full_name"`
	Phone     *string `json:"phone"`
	Password  *string `json:"password"`
	Role      *string `json:"role"`
	ManagerID *int64  `json:"manager_id"`
	Active    *bool   `json:"active"`
}

// UpdateUser modifies a user. Admin/manager may edit anyone; an employee may
// edit only their own profile fields (not role/active). Manager cannot promote
// users to admin.
func (h *handler) UpdateUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	role := sessionRole(c)
	self := GetSessionInfo(c).id
	isPriv := role == "admin" || role == "manager"
	if !isPriv && id != self {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Self-edit cannot change role / active.
	if !isPriv {
		req.Role = nil
		req.Active = nil
		req.ManagerID = nil
	}
	// Manager cannot promote to admin.
	if role == "manager" && req.Role != nil && *req.Role == "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "managers may not assign admin role"})
		return
	}
	if req.Role != nil {
		if _, ok := validRoles[*req.Role]; !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
			return
		}
	}

	// Translate the optional pointer fields into SQL-safe COALESCE inputs.
	// nil pointer ⇒ pass nil (driver -> NULL) ⇒ COALESCE keeps the existing value.
	hasUpdate := false
	var (
		emailArg     any = nil
		fullNameArg  any = nil
		phoneArg     any = nil
		passwordArg  any = nil
		roleArg      any = nil
		managerIDArg any = nil
		activeArg    any = nil
	)
	if req.Email != nil {
		emailArg = nullableString(*req.Email)
		hasUpdate = true
	}
	if req.FullName != nil {
		fullNameArg = nullableString(*req.FullName)
		hasUpdate = true
	}
	if req.Phone != nil {
		phoneArg = nullableString(*req.Phone)
		hasUpdate = true
	}
	if req.Password != nil && *req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("UpdateUser bcrypt: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
			return
		}
		passwordArg = string(hash)
		hasUpdate = true
	}
	if req.Role != nil {
		roleArg = *req.Role
		hasUpdate = true
	}
	if req.ManagerID != nil {
		managerIDArg = *req.ManagerID
		hasUpdate = true
	}
	if req.Active != nil {
		activeArg = boolToInt(*req.Active)
		hasUpdate = true
	}
	if !hasUpdate {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}

	const updateSQL = `UPDATE user SET
            email = COALESCE(?, email),
            full_name = COALESCE(?, full_name),
            phone = COALESCE(?, phone),
            password = COALESCE(?, password),
            role = COALESCE(?, role),
            manager_id = COALESCE(?, manager_id),
            is_active = COALESCE(?, is_active)
        WHERE id = ? AND is_deleted = 0`
	if _, err := h.DB.Exec(updateSQL,
		emailArg, fullNameArg, phoneArg, passwordArg,
		roleArg, managerIDArg, activeArg, id,
	); err != nil {
		log.Printf("UpdateUser: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update user"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id})
}

// ──────────────────────────────────────────────────────────────────────────────
// DELETE /api/v2/users/:id
// ──────────────────────────────────────────────────────────────────────────────

// DeleteUser soft-deletes a user. Admin only. Refuses self-delete.
func (h *handler) DeleteUser(c *gin.Context) {
	if !requireRole(c, "admin") {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if id == GetSessionInfo(c).id {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete yourself"})
		return
	}

	res, err := h.DB.Exec(
		"UPDATE user SET is_deleted = 1, is_active = 0 WHERE id = ? AND is_deleted = 0",
		id,
	)
	if err != nil {
		log.Printf("DeleteUser: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete user"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	// Best-effort: kill all sessions for this user.
	if _, err := h.DB.Exec("DELETE FROM refresh_token WHERE user_id = ?", id); err != nil {
		log.Printf("DeleteUser revoke tokens: %v", err)
	}
	c.JSON(http.StatusOK, gin.H{"id": id})
}

// ──────────────────────────────────────────────────────────────────────────────
// PUT /api/v2/users/:id/active
// ──────────────────────────────────────────────────────────────────────────────

// ToggleUserActive sets is_active. Admin only.
func (h *handler) ToggleUserActive(c *gin.Context) {
	if !requireRole(c, "admin") {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		Active bool `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if _, err := h.DB.Exec(
		"UPDATE user SET is_active = ? WHERE id = ? AND is_deleted = 0",
		boolToInt(req.Active), id,
	); err != nil {
		log.Printf("ToggleUserActive: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update active flag"})
		return
	}
	if !req.Active {
		_, _ = h.DB.Exec("DELETE FROM refresh_token WHERE user_id = ?", id)
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "active": req.Active})
}

// ──────────────────────────────────────────────────────────────────────────────
// PUT /api/v2/users/:id/permissions
// ──────────────────────────────────────────────────────────────────────────────

type permPayload struct {
	Resource string   `json:"resource"`
	Actions  []string `json:"actions"`
}

type permissionsRequest struct {
	Permissions []permPayload `json:"permissions"`
}

// UpdateUserPermissions replaces the per-resource action grants for a user.
// Admin only. Atomic: deletes existing rows then inserts the new set in a tx.
func (h *handler) UpdateUserPermissions(c *gin.Context) {
	if !requireRole(c, "admin") {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req permissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	// Validate input before mutating anything.
	type rowFlags struct{ canView, canAdd, canEdit, canDel bool }
	rows := make(map[string]rowFlags, len(req.Permissions))
	for _, p := range req.Permissions {
		if _, ok := knownPermResources[p.Resource]; !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown resource: " + p.Resource})
			return
		}
		rf := rows[p.Resource]
		for _, a := range p.Actions {
			if _, ok := validActions[a]; !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid action: " + a})
				return
			}
			switch a {
			case "view":
				rf.canView = true
			case "add":
				rf.canAdd = true
			case "edit":
				rf.canEdit = true
			case "delete":
				rf.canDel = true
			}
		}
		rows[p.Resource] = rf
	}

	// Make sure user exists (and isn't deleted).
	var exists int
	if err := h.DB.QueryRow(
		"SELECT COUNT(*) FROM user WHERE id = ? AND is_deleted = 0", id,
	).Scan(&exists); err != nil || exists == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	tx, err := h.DB.Begin()
	if err != nil {
		log.Printf("UpdateUserPermissions begin: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start tx"})
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM user_permission WHERE user_id = ?", id); err != nil {
		log.Printf("UpdateUserPermissions delete: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to clear permissions"})
		return
	}
	stmt, err := tx.Prepare(`
        INSERT INTO user_permission (user_id, resource, can_view, can_add, can_edit, can_delete)
        VALUES (?,?,?,?,?,?)`)
	if err != nil {
		log.Printf("UpdateUserPermissions prepare: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare insert"})
		return
	}
	defer stmt.Close()
	for resource, rf := range rows {
		if _, err := stmt.Exec(id, resource,
			boolToInt(rf.canView), boolToInt(rf.canAdd),
			boolToInt(rf.canEdit), boolToInt(rf.canDel)); err != nil {
			log.Printf("UpdateUserPermissions insert: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert permissions"})
			return
		}
	}
	if err := tx.Commit(); err != nil {
		log.Printf("UpdateUserPermissions commit: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to commit"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "count": len(rows)})
}

// ──────────────────────────────────────────────────────────────────────────────
// Local helpers
// ──────────────────────────────────────────────────────────────────────────────

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
