package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// FingerprintVersion is the version of the request fingerprint. Changing what a fingerprint covers needs a new version,
// so a stored fingerprint is never compared with one that was computed another way.
const FingerprintVersion = 1

// Fingerprint returns the SHA-256 of the JSON encoding of v, as 64 lowercase hex digits. Struct fields encode in their
// declaration order, so one value always gives one fingerprint.
func Fingerprint(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("identity: fingerprint: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
