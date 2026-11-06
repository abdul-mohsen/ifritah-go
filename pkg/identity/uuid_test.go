package identity

import (
	"regexp"
	"strings"
	"testing"
)

var uuidLayout = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func TestNewUUIDIsAVersion4LowercaseUUID(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id, err := NewUUID()
		if err != nil {
			t.Fatal(err)
		}
		if !uuidLayout.MatchString(id) || id[14] != '4' || !strings.ContainsRune("89ab", rune(id[19])) {
			t.Fatalf("uuid %q is not a lowercase version 4 UUID", id)
		}
		if seen[id] {
			t.Fatalf("uuid %q was returned twice", id)
		}
		seen[id] = true
	}
}
