package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// abortJSON ends the request with the given status and a uniform JSON body so
// the frontend can show a useful message instead of an empty body.
//
// Body shape: {"detail": "<msg>", "error": "<status text>", "status": <code>}.
// All handlers should use this (or respondJSON) instead of writing ad-hoc
// gin.H{"error": ...} payloads.
func abortJSON(c *gin.Context, status int, detail string) {
	c.AbortWithStatusJSON(status, gin.H{
		"detail": detail,
		"error":  http.StatusText(status),
		"status": status,
	})
}

// respondJSON writes a non-aborting JSON response with the same envelope as
// abortJSON. Useful for handler error paths that should not call further
// middleware but also do not need to abort the chain.
func respondJSON(c *gin.Context, status int, detail string) {
	c.JSON(status, gin.H{
		"detail": detail,
		"error":  http.StatusText(status),
		"status": status,
	})
}

// extractBearerToken pulls the token out of an "Authorization: Bearer <tok>"
// header. Returns ("", false) when the header is missing, empty, lacks the
// "Bearer " prefix (case-sensitive per RFC 6750 §2.1), or has an empty token.
//
// Hardens the previous strings.SplitN approach which silently accepted
// payloads like "X-Bearer foo" because SplitN matches the literal anywhere
// in the string.
func extractBearerToken(authHeader string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) {
		return "", false
	}
	tok := strings.TrimSpace(authHeader[len(prefix):])
	if tok == "" {
		return "", false
	}
	return tok, true
}
