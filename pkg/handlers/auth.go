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
	"strings"
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
	if err := c.BindJSON(&request); err != nil {
		log.Panic(err)
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
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "invalid credentials"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "database error"})
		log.Panic(err)
	}

	if !isActive {
		c.JSON(http.StatusForbidden, gin.H{"detail": "account is deactivated"})
		log.Panic("not activate user")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(request.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "invalid credentials"})
		log.Panic(err)
	}

	accessToken, err := GenerateAccessToken(userID, request.Username, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate access token"})
		log.Panic(err)
	}

	refreshToken, err := GenerateRefreshToken(userID, request.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate refresh token"})
		log.Panic(err)
	}

	// Store refresh token hash in DB for revocation support
	tokenHash := sha256Hex(refreshToken)
	_, _ = h.DB.Exec(
		"INSERT INTO refresh_token (user_id, token_hash, device_name, ip_address, expires_at) VALUES (?, ?, ?, ?, ?)",
		userID, tokenHash,
		c.GetHeader("User-Agent"), c.ClientIP(),
		time.Now().Add(model.JWTSettings.RefreshExpiration),
	)

	// Update last_login
	_, _ = h.DB.Exec("UPDATE user SET last_login = NOW() WHERE id = ?", userID)

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

func JWTVerifyMiddleware(c *gin.Context) {
	// Get the JWT token from the Authorization header
	fullTokenString := c.GetHeader("Authorization")
	split := strings.Split(fullTokenString, "Bearer ")
	if len(split) != 2 {
		c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("no access token found"))
	}

	tokenString := split[1]

	// Define the secret key used to sign the token
	secretKey := []byte(model.JWTSettings.JWTSecertKey)
	token, err := jwt.ParseWithClaims(tokenString, &model.Claims{},
		func(token *jwt.Token) (any, error) {
			// Verify the signing method
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
			}

			// Return the secret key
			return []byte(secretKey), nil
		})

	if err != nil {
		if err == jwt.ErrSignatureInvalid {
			c.Status(http.StatusUnauthorized)
			return
		}
		c.Status(http.StatusBadRequest)
		return
	}
	if claims, ok := token.Claims.(*model.Claims); ok && token.Valid {

		if !time.Unix(claims.Expiration, 0).Before(time.Now()) {
			// Store the decoded JWT in the context for later use
			c.Set("decoded_jwt", claims)

			// Continue the request processing
			c.Next()
			return
		}
	}

	c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("Token is invalid"))
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
	authHeader := c.GetHeader("Authorization")
	parts := strings.SplitN(authHeader, "Bearer ", 2)
	if len(parts) != 2 || parts[1] == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "no refresh token provided"})
		return
	}
	tokenString := parts[1]

	// Parse and validate the refresh token
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(model.JWTSettings.JWTSecertKey), nil
	})
	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "invalid or expired refresh token"})
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "invalid token claims"})
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
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "invalid token claims"})
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
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "user not found or inactive"})
		return
	}

	// Generate new tokens (token rotation)
	newAccessToken, err := GenerateAccessToken(userID, username, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to generate access token"})
		return
	}
	newRefreshToken, err := GenerateRefreshToken(userID, username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to generate refresh token"})
		return
	}

	// Rotate: update the session with new tokens
	newHash := sha256Hex(newRefreshToken)
	_, _ = h.DB.Exec(
		"UPDATE refresh_token SET token_hash = ?, expires_at = ? WHERE id = ?",
		newHash,
		time.Now().Add(model.JWTSettings.RefreshExpiration),
		sessionID,
	)

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
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid request: " + err.Error()})
		return
	}

	// Check username not already taken
	var exists int
	h.DB.QueryRow("SELECT COUNT(*) FROM user WHERE username = ?", req.Username).Scan(&exists)
	if exists > 0 {
		c.JSON(http.StatusConflict, gin.H{"detail": "username already exists"})
		return
	}

	// Check email not already taken
	h.DB.QueryRow("SELECT COUNT(*) FROM user WHERE email = ?", req.Email).Scan(&exists)
	if exists > 0 {
		c.JSON(http.StatusConflict, gin.H{"detail": "email already exists"})
		return
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to hash password"})
		log.Panic(err)
	}

	// Insert user with default role "employee"
	result, err := h.DB.Exec(
		"INSERT INTO user (username, email, password, full_name, phone, role, is_active) VALUES (?, ?, ?, ?, ?, 'employee', 1)",
		req.Username, req.Email, string(hash), req.FullName, req.Phone,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to create user"})
		log.Panic(err)
	}

	userID, _ := result.LastInsertId()

	// Seed default permissions for the new user (view-only on most resources)
	defaultPerms := []string{"invoices", "products", "clients", "suppliers", "stores", "orders"}
	for _, resource := range defaultPerms {
		_, _ = h.DB.Exec(
			"INSERT INTO permissions (user_id, resource, can_view, can_add, can_edit, can_delete) VALUES (?, ?, 1, 0, 0, 0)",
			userID, resource,
		)
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
		// User not found — still return 200 to prevent email enumeration
		c.JSON(http.StatusOK, gin.H{"detail": "if the email exists, a reset link has been sent"})
		return
	}

	// Generate secure reset token
	tokenBytes := make([]byte, 64)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to generate token"})
		return
	}
	resetToken := hex.EncodeToString(tokenBytes)

	// Store hashed token in DB (expires in 1 hour)
	tokenHash := sha256Hex(resetToken)
	_, _ = h.DB.Exec(
		"INSERT INTO password_reset_tokens (user_id, token, expires_at) VALUES (?, ?, ?)",
		userID, tokenHash, time.Now().Add(1*time.Hour),
	)

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
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid or expired reset token"})
		return
	}

	// Hash new password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to hash password"})
		return
	}

	// Update password
	_, err = h.DB.Exec("UPDATE user SET password = ? WHERE id = ?", string(hash), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to update password"})
		return
	}

	// Mark token as used
	_, _ = h.DB.Exec("UPDATE password_reset_tokens SET used_at = NOW() WHERE id = ?", tokenID)

	// Invalidate all sessions for this user (force re-login)
	_, _ = h.DB.Exec("DELETE FROM sessions WHERE user_id = ?", userID)

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

	// Delete all sessions for this user
	_, _ = h.DB.Exec("DELETE FROM refresh_token WHERE user_id = ?", userID)

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

	// Fetch permissions
	rows, err := h.DB.Query(
		"SELECT resource, can_view, can_add, can_edit, can_delete FROM permissions WHERE user_id = ?",
		userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "failed to fetch permissions"})
		return
	}
	defer rows.Close()

	type Permission struct {
		Resource string   `json:"resource"`
		Actions  []string `json:"actions"`
	}
	var permissions []Permission
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
