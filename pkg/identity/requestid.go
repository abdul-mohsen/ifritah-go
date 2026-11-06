package identity

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// maxPosIDLen matches bill.pos_id, a VARCHAR(64) counted in characters.
const maxPosIDLen = 64

var lowerUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// ValidatePosID checks the point-of-sale id of a final sale: 1 to 64 characters, with no spaces at
// either end.
func ValidatePosID(posID string) error {
	if posID == "" || strings.TrimSpace(posID) != posID || utf8.RuneCountInString(posID) > maxPosIDLen {
		return fmt.Errorf("pos_id must be 1 to %d characters with no spaces at either end", maxPosIDLen)
	}
	return nil
}

// ValidateBillRequestID checks the id that a point of sale gives each sale attempt. It is a lowercase
// UUID, and a retry of the same attempt sends the same id.
func ValidateBillRequestID(requestID string) error {
	if !lowerUUID.MatchString(requestID) {
		return fmt.Errorf("bill_request_id must be a lowercase UUID")
	}
	return nil
}
