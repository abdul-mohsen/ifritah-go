package outbox

import (
	"strings"
	"testing"
)

func TestNewEventIDIsAVersion4LowercaseUUID(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id, err := NewEventID()
		if err != nil {
			t.Fatal(err)
		}
		if !uuidPattern.MatchString(id) || id[14] != '4' || !strings.ContainsRune("89ab", rune(id[19])) {
			t.Fatalf("event id %q is not a lowercase version 4 UUID", id)
		}
		if seen[id] {
			t.Fatalf("event id %q was returned twice", id)
		}
		seen[id] = true
	}
}
