package handlers

import (
	"strconv"
	"strings"
)

// parseInt32Param parses s as a base-10 signed integer that is guaranteed to
// fit in 32 bits, returning an error for out-of-range or malformed input
// instead of silently truncating it. Use this — not strconv.Atoi/ParseInt
// followed by a manual int32(...) cast — for any :id-style path/query
// parameter that is ultimately stored or compared as int32
// (CodeQL go/incorrect-integer-conversion).
func parseInt32Param(s string) (int32, error) {
	v, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(v), nil
}

// parseUint32Param parses s as a base-10 unsigned integer that is guaranteed
// to fit in 32 bits, returning an error for negative, out-of-range or
// malformed input instead of silently wrapping it. Use this — not
// strconv.Atoi/ParseInt followed by a manual uint32(...) cast — for any
// :id-style path/query parameter that is ultimately stored or compared as
// uint32 (CodeQL go/incorrect-integer-conversion).
func parseUint32Param(s string) (uint32, error) {
	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(v), nil
}

// sanitizeForLog strips CR/LF characters from strings that may be
// influenced by request input before they are written to application logs,
// preventing log forging via embedded control characters
// (CodeQL go/log-injection).
func sanitizeForLog(s string) string {
	return strings.NewReplacer("\n", " ", "\r", " ").Replace(s)
}
