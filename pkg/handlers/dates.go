package handlers

import "time"

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
