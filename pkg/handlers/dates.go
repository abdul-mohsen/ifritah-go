package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// parseFlexibleDate parses a date string accepting either RFC3339
// (e.g. "2025-04-30T00:00:00Z") or the date-only form "2006-01-02".
// Returns the parsed time and the error from the last attempt if
// neither format matches.
func parseFlexibleDate(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Parse(time.DateOnly, s)
}

// parseOptionalRequestDate parses an optional date pointer field from a
// request payload. On parse error it writes a 400 JSON response naming the
// field and returns ok=false so the caller can `return`. When raw is nil,
// it returns (nil, true) to mean "field not supplied".
func parseOptionalRequestDate(c *gin.Context, raw *string, field string) (*time.Time, bool) {
	if raw == nil {
		return nil, true
	}
	t, err := parseFlexibleDate(*raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + field})
		return nil, false
	}
	return &t, true
}
