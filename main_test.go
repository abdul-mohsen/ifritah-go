package main

import "testing"

// TestPlaceholder keeps `go test ./...` green until proper integration tests
// are added. The previous stub referenced an undefined `setupRouter()` and was
// missing `import "testing"`, which broke CI on every PR.
func TestPlaceholder(t *testing.T) {
	t.Log("placeholder test — replace with real coverage")
}
