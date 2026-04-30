package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"ifritah/web-service-gin/pkg/model"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func GenerateAccessToken(userID int64, username, role string) (string, error) {
	claims := jwt.MapClaims{
		"userId":   userID,
		"username": username,
		"role":     role,
		"exp":      time.Now().Add(model.JWTSettings.AccessExpiration).Unix(),
		"iat":      time.Now().Unix(),
		"type":     "access",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	return token.SignedString([]byte(model.JWTSettings.JWTSecertKey))
}

// GenerateRefreshToken creates a long-lived refresh token.
func GenerateRefreshToken(userID int64, username string) (string, error) {
	claims := jwt.MapClaims{
		"userId":   userID,
		"username": username,
		"exp":      time.Now().Add(model.JWTSettings.RefreshExpiration).Unix(),
		"iat":      time.Now().Unix(),
		"type":     "refresh",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	return token.SignedString([]byte(model.JWTSettings.JWTSecertKey))
}

func checkPassword(hashedPassword []byte, password string) error {
	return bcrypt.CompareHashAndPassword(hashedPassword, []byte(password))
}

func (h *handler) Login(c *gin.Context) {

	var request model.LoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": invalidRequestDetail(err)})
		return
	}

	// Look up user by username
	var userID int64
	var passwordHash, role string
	var isActive bool
	err := h.DB.QueryRow(
		"SELECT id, password, role, is_active FROM user WHERE username = ? LIMIT 1",
		request.Username,
	).Scan(&userID, &passwordHash, &role, &isActive)

	if err == sql.ErrNoRows {
		// Constant-time comparison to prevent timing attacks
		bcrypt.CompareHashAndPassword([]byte("$2a$12$dummy.hash.for.timing.attack.prevention.xxxxx"), []byte(request.Password))
		c.JSON(http.StatusUnauthorized, gin.H{"detail": ErrInvalidCredentials})
		return
	}
	if err != nil {
		log.Printf("login db error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": ErrDatabase})
		return
	}

	if !isActive {
		c.JSON(http.StatusForbidden, gin.H{"detail": ErrAccountDeactivated})
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(request.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": ErrInvalidCredentials})
		return
	}

	accessToken, err := GenerateAccessToken(userID, request.Username, role)
	if err != nil {
		log.Printf("access token error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": ErrGenerateAccessTok})
		return
	}

	refreshToken, err := GenerateRefreshToken(userID, request.Username)
	if err != nil {
		log.Printf("refresh token error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": ErrGenerateRefreshTok})
		return
	}

	// Store refresh token hash in DB for revocation support.
	// Best-effort: failure here doesn't block login but is logged so we can
	// detect refresh-token storage outages.
	tokenHash := sha256Hex(refreshToken)
	if _, err := h.DB.Exec(
		"INSERT INTO refresh_token (user_id, token_hash, device_name, ip_address, expires_at) VALUES (?, ?, ?, ?, ?)",
		userID, tokenHash,
		c.GetHeader("User-Agent"), c.ClientIP(),
		time.Now().Add(model.JWTSettings.RefreshExpiration),
	); err != nil {
		log.Printf("Login: store refresh token: %v", err)
	}

	// Update last_login (best-effort).
	if _, err := h.DB.Exec("UPDATE user SET last_login = NOW() WHERE id = ?", userID); err != nil {
		log.Printf("Login: update last_login: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

// generateSessionID creates a cryptographically random session ID.
func generateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "s_" + hex.EncodeToString(b)
}

// abortJSON / extractBearerToken now live in pkg/handlers/responses.go.

func JWTVerifyMiddleware(c *gin.Context) {
	tokenString, ok := extractBearerToken(c.GetHeader("Authorization"))
	if !ok {
		abortJSON(c, http.StatusUnauthorized, ErrMissingAuthHeader)
		return
	}

	secretKey := []byte(model.JWTSettings.JWTSecertKey)
	token, err := jwt.ParseWithClaims(tokenString, &model.Claims{},
		func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return secretKey, nil
		})
	if err != nil || token == nil || !token.Valid {
		if err != nil {
			log.Printf("JWTVerifyMiddleware: parse token: %v", err)
		}
		abortJSON(c, http.StatusUnauthorized, ErrInvalidOrExpired)
		return
	}

	claims, ok := token.Claims.(*model.Claims)
	if !ok {
		abortJSON(c, http.StatusUnauthorized, ErrInvalidTokenClaims)
		return
	}

	if claims.Expiration > 0 && time.Unix(claims.Expiration, 0).Before(time.Now()) {
		abortJSON(c, http.StatusUnauthorized, ErrTokenExpired)
		return
	}

	// Expose claims for handlers and downstream middleware (e.g. RequireRole).
	c.Set("decoded_jwt", claims)
	c.Set("user_id", claims.Id)
	c.Set("userId", claims.Id) // back-compat for handlers using GetInt64("userId")
	c.Set("username", claims.Username)
	c.Set("user_role", claims.Role)

	c.Next()
}

// ============================================================================
// Role-based access control
// ============================================================================

// RoleAdmin / RoleManager / RoleEmployee match the enum on the user table.
const (
	RoleAdmin    = "admin"
	RoleManager  = "manager"
	RoleEmployee = "employee"
)

// RequireRole returns a middleware that allows the request through only if the
// authenticated user's role is in the allowed list. The JWT middleware must run
// first so that "user_role" is set on the context.
func RequireRole(allowed ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("user_role")
		roleStr, _ := role.(string)
		for _, a := range allowed {
			if a == roleStr {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"detail":         "insufficient permissions",
			"error":          "forbidden",
			"status":         http.StatusForbidden,
			"required_roles": allowed,
			"your_role":      roleStr,
		})
	}
}

// RequireAdmin is a shorthand for admin-only routes.
func RequireAdmin() gin.HandlerFunc { return RequireRole(RoleAdmin) }

// RequireManagerOrAbove allows admin and manager.
func RequireManagerOrAbove() gin.HandlerFunc {
	return RequireRole(RoleAdmin, RoleManager)
}

// ============================================================================
// Token Refresh
// ============================================================================

// Refresh generates a new access token using a valid refresh token.
// POST /api/v2/refresh
//
// Request:  Authorization: Bearer <refresh_token>
// Response: {"access_token":"...","refresh_token":"..."}
//
// This MUST be in the non-authenticated route group (nonAuthGroup)
// because the access token is expired when this is called.
func (h *handler) Refresh(c *gin.Context) {
	// Extract refresh token from Authorization header
	tokenString, ok := extractBearerToken(c.GetHeader("Authorization"))
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "no refresh token provided"})
		return
	}

	// Parse and validate the refresh token
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(model.JWTSettings.JWTSecertKey), nil
	})
	if err != nil || !token.Valid {
		if err != nil {
			log.Printf("Refresh: parse refresh token: %v", err)
		}
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "invalid or expired refresh token"})
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": ErrInvalidTokenClaims})
		return
	}

	// Verify this is a refresh token, not an access token
	tokenType, _ := claims["type"].(string)
	if tokenType != "refresh" {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "not a refresh token"})
		return
	}

	username, _ := claims["username"].(string)
	userIDFloat, _ := claims["userId"].(float64)
	userID := int64(userIDFloat)

	if username == "" || userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": ErrInvalidTokenClaims})
		return
	}

	// Verify the refresh token exists in DB and is not expired
	tokenHash := sha256Hex(tokenString)
	var sessionID string
	err = h.DB.QueryRow(
		"SELECT id FROM refresh_token WHERE token_hash = ? AND revoked = 0 AND expires_at > NOW() LIMIT 1",
		tokenHash,
	).Scan(&sessionID)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("Refresh: lookup refresh token: %v", err)
		}
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "refresh token revoked or not found"})
		return
	}

	// Verify the user still exists and is active
	var role string
	var isActive bool
	err = h.DB.QueryRow(
		"SELECT role, is_active FROM user WHERE id = ? LIMIT 1",
		userID,
	).Scan(&role, &isActive)
	if err != nil || !isActive {
		if err != nil && err != sql.ErrNoRows {
			log.Printf("Refresh: lookup user: %v", err)
		}
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "user not found or inactive"})
		return
	}

	// Generate new tokens (token rotation)
	newAccessToken, err := GenerateAccessToken(userID, username, role)
	if err != nil {
		log.Printf("Refresh: generate access token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": ErrGenerateAccessTok})
		return
	}
	newRefreshToken, err := GenerateRefreshToken(userID, username)
	if err != nil {
		log.Printf("Refresh: generate refresh token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": ErrGenerateRefreshTok})
		return
	}

	// Rotate: update the session with new tokens
	newHash := sha256Hex(newRefreshToken)
	if _, err := h.DB.Exec(
		"UPDATE refresh_token SET token_hash = ?, expires_at = ? WHERE id = ?",
		newHash,
		time.Now().Add(model.JWTSettings.RefreshExpiration),
		sessionID,
	); err != nil {
		log.Printf("Refresh: rotate refresh token: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  newAccessToken,
		"refresh_token": newRefreshToken,
	})
}

// ============================================================================
// Register
// ============================================================================

// RegisterRequest defines the registration payload.
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	FullName string `json:"full_name"`
	Phone    string `json:"phone"`
}

// Register creates a new user account.
// POST /api/v2/register
//
// Request:  {"username":"...","email":"...","password":"...","full_name":"...","phone":"..."}
// Response: {"id":1,"username":"...","email":"...","role":"employee"}
func (h *handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": invalidRequestDetail(err)})
		return
	}

	// Check username not already taken
	var exists int
	if err := h.DB.QueryRow("SELECT COUNT(*) FROM user WHERE username = ?", req.Username).Scan(&exists); err != nil {
		log.Printf("Register: username uniqueness check: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": ErrDatabase})
		return
	}
	if exists > 0 {
		c.JSON(http.StatusConflict, gin.H{"detail": ErrUsernameExists})
		return
	}

	// Check email not already taken
	if err := h.DB.QueryRow("SELECT COUNT(*) FROM user WHERE email = ?", req.Email).Scan(&exists); err != nil {
		log.Printf("Register: email uniqueness check: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": ErrDatabase})
		return
	}
	if exists > 0 {
		c.JSON(http.StatusConflict, gin.H{"detail": ErrEmailExists})
		return
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Register: hash password: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": ErrHashPassword})
		return
	}

	// Insert user with default role "employee"
	result, err := h.DB.Exec(
		"INSERT INTO user (username, email, password, full_name, phone, role, is_active) VALUES (?, ?, ?, ?, ?, 'employee', 1)",
		req.Username, req.Email, string(hash), req.FullName, req.Phone,
	)
	if err != nil {
		log.Printf("Register: insert user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": ErrCreateUser})
		return
	}

	userID, _ := result.LastInsertId()

	// Seed default permissions for the new user (view-only on most resources).
	// Best-effort: failures are logged but don't fail the registration.
	defaultPerms := []string{"invoices", "products", "clients", "suppliers", "stores", "orders"}
	for _, resource := range defaultPerms {
		if _, err := h.DB.Exec(
			"INSERT INTO user_permission (user_id, resource, can_view, can_add, can_edit, can_delete) VALUES (?, ?, 1, 0, 0, 0)",
			userID, resource,
		); err != nil {
			log.Printf("Register: seed permission %q for user %d: %v", resource, userID, err)
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":       userID,
		"username": req.Username,
		"email":    req.Email,
		"role":     "employee",
	})
}

// ============================================================================
// Forgot / Reset Password
// ============================================================================

// ForgotPassword initiates a password reset flow.
// POST /api/v2/forgot-password
//
// Request:  {"email":"user@example.com"}
// Response: 200 OK always (prevent email enumeration)
func (h *handler) ForgotPassword(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid email"})
		return
	}

	// Look up user — but always return 200 regardless
	var userID int64
	err := h.DB.QueryRow("SELECT id FROM user WHERE email = ? AND is_active = 1", req.Email).Scan(&userID)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("ForgotPassword: lookup user: %v", err)
		}
		// User not found — still return 200 to prevent email enumeration
		c.JSON(http.StatusOK, gin.H{"detail": "if the email exists, a reset link has been sent"})
		return
	}

	// Generate secure reset token
	tokenBytes := make([]byte, 64)
	if _, err := rand.Read(tokenBytes); err != nil {
		log.Printf("ForgotPassword: rand.Read: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to generate token"})
		return
	}
	resetToken := hex.EncodeToString(tokenBytes)

	// Store hashed token in DB (expires in 1 hour)
	tokenHash := sha256Hex(resetToken)
	if _, err := h.DB.Exec(
		"INSERT INTO password_reset_tokens (user_id, token, expires_at) VALUES (?, ?, ?)",
		userID, tokenHash, time.Now().Add(1*time.Hour),
	); err != nil {
		log.Printf("ForgotPassword: insert reset token: %v", err)
	}

	// TODO: Send email with reset link containing the raw resetToken
	// For now, log it (remove in production)
	fmt.Printf("[FORGOT-PASSWORD] Reset token for user %d: %s\n", userID, resetToken)

	c.JSON(http.StatusOK, gin.H{"detail": "if the email exists, a reset link has been sent"})
}

// ResetPassword sets a new password using a valid reset token.
// POST /api/v2/reset-password
//
// Request:  {"token":"hex_token","new_password":"NewPass123!"}
// Response: 200 OK or 400 if invalid/expired
func (h *handler) ResetPassword(c *gin.Context) {
	var req struct {
		Token       string `json:"token"        binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request"})
		return
	}

	// Look up token (stored as hash)
	tokenHash := sha256Hex(req.Token)
	var tokenID int64
	var userID int64
	err := h.DB.QueryRow(
		"SELECT id, user_id FROM password_reset_tokens WHERE token = ? AND expires_at > NOW() AND used_at IS NULL LIMIT 1",
		tokenHash,
	).Scan(&tokenID, &userID)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("ResetPassword: lookup reset token: %v", err)
		}
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid or expired reset token"})
		return
	}

	// Hash new password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("ResetPassword: hash password: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": ErrHashPassword})
		return
	}

	// Update password
	_, err = h.DB.Exec("UPDATE user SET password = ? WHERE id = ?", string(hash), userID)
	if err != nil {
		log.Printf("ResetPassword: update password: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": ErrUpdatePassword})
		return
	}

	// Mark token as used (best-effort)
	if _, err := h.DB.Exec("UPDATE password_reset_tokens SET used_at = NOW() WHERE id = ?", tokenID); err != nil {
		log.Printf("ResetPassword: mark token used: %v", err)
	}

	// Invalidate all sessions for this user (force re-login)
	if _, err := h.DB.Exec("DELETE FROM sessions WHERE user_id = ?", userID); err != nil {
		log.Printf("ResetPassword: delete sessions: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"detail": "password updated successfully"})
}

// ============================================================================
// Logout
// ============================================================================

// Logout invalidates the current session.
// POST /api/v2/logout
//
// Request:  Authorization: Bearer <access_token>
// Response: 200 OK
func (h *handler) Logout(c *gin.Context) {
	userID := c.GetInt64("userId") // from JWT middleware

	// Delete all sessions for this user (best-effort).
	if _, err := h.DB.Exec("DELETE FROM refresh_token WHERE user_id = ?", userID); err != nil {
		log.Printf("Logout: delete refresh tokens for user %d: %v", userID, err)
	}

	c.JSON(http.StatusOK, gin.H{"detail": "logged out"})
}

// ============================================================================
// Get Current User Profile (with permissions)
// ============================================================================

// GetMe returns the currently authenticated user's profile and permissions.
// GET /api/v2/users/me
//
// Request:  Authorization: Bearer <access_token>
// Response: {"id":1,"username":"ssda","email":"...","role":"admin","permissions":[...]}
//
// The frontend uses this to populate RBAC — currently mocked because this
// endpoint doesn't exist.
func (h *handler) GetMe(c *gin.Context) {
	userID := c.GetInt64("userId")

	var user struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
		FullName string `json:"full_name"`
		Phone    string `json:"phone"`
		Role     string `json:"role"`
		IsActive bool   `json:"is_active"`
	}
	err := h.DB.QueryRow(
		"SELECT id, username, email, COALESCE(full_name,'') as full_name, COALESCE(phone,'') as phone, role, is_active FROM user WHERE id = ?",
		userID,
	).Scan(&user.ID, &user.Username, &user.Email, &user.FullName, &user.Phone, &user.Role, &user.IsActive)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "user not found"})
		return
	}

	// Fetch permissions (best-effort — table name varies between deploys).
	type Permission struct {
		Resource string   `json:"resource"`
		Actions  []string `json:"actions"`
	}
	permissions := []Permission{}
	rows, err := h.DB.Query(
		"SELECT resource, can_view, can_add, can_edit, can_delete FROM user_permission WHERE user_id = ?",
		userID,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var resource string
			var canView, canAdd, canEdit, canDelete bool
			rows.Scan(&resource, &canView, &canAdd, &canEdit, &canDelete)
			var actions []string
			if canView {
				actions = append(actions, "view")
			}
			if canAdd {
				actions = append(actions, "add")
			}
			if canEdit {
				actions = append(actions, "edit")
			}
			if canDelete {
				actions = append(actions, "delete")
			}
			permissions = append(permissions, Permission{Resource: resource, Actions: actions})
		}
	} else {
		log.Printf("GetMe permissions query failed (non-fatal): %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"id":          user.ID,
		"username":    user.Username,
		"email":       user.Email,
		"full_name":   user.FullName,
		"phone":       user.Phone,
		"role":        user.Role,
		"is_active":   user.IsActive,
		"permissions": permissions,
	})
}

func GetSessionInfo(c *gin.Context) userSession {

	claimsStr, exist := c.Get("decoded_jwt")
	if !exist {
		c.AbortWithStatus(http.StatusUnauthorized)
	}
	claims := claimsStr.(*model.Claims)
	user := userSession{
		id:       claims.Id,
		username: claims.Username,
		exp:      claims.Expiration,
	}
	return user
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
