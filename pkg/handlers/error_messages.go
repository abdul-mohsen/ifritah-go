package handlers

// Common error / response detail strings used across handlers. Centralized
// here so the frontend sees consistent wording and we don't drift on copy.
//
// Add new entries sparingly: only when the same literal would appear in
// three or more sites, or when the message is part of a documented contract
// with the frontend.
const (
	// 400 Bad Request
	ErrInvalidRequest    = "invalid request"
	ErrInvalidUserID     = "invalid user id"
	ErrInvalidRole       = "invalid role"
	ErrNoFieldsToUpdate  = "no fields to update"

	// 401 Unauthorized
	ErrInvalidCredentials = "invalid credentials"
	ErrInvalidTokenClaims = "invalid token claims"
	ErrInvalidOrExpired   = "invalid or expired token"
	ErrMissingAuthHeader  = "missing or invalid authorization header"
	ErrTokenExpired       = "token expired"

	// 403 Forbidden
	ErrAccountDeactivated = "account is deactivated"

	// 404 Not Found
	ErrUserNotFound = "user not found"

	// 409 Conflict
	ErrUsernameExists = "username already exists"
	ErrEmailExists    = "email already exists"

	// 500 Internal Server Error
	ErrDatabase            = "database error"
	ErrHashPassword        = "failed to hash password"
	ErrCreateUser          = "failed to create user"
	ErrUpdateUser          = "failed to update user"
	ErrDeleteUser          = "failed to delete user"
	ErrFetchUser           = "failed to fetch user"
	ErrListUsers           = "failed to list users"
	ErrResetPassword       = "failed to reset password"
	ErrUpdatePassword      = "failed to update password"
	ErrGenerateAccessTok   = "could not generate access token"
	ErrGenerateRefreshTok  = "could not generate refresh token"
)

// invalidRequestDetail returns the canonical "invalid request" string with
// the underlying binding error appended. Use for 400 responses on
// ShouldBindJSON failures so the frontend can display the validation tail.
func invalidRequestDetail(err error) string {
	if err == nil {
		return ErrInvalidRequest
	}
	return ErrInvalidRequest + ": " + err.Error()
}
