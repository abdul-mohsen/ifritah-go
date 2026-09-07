package buildinfo

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCurrentLoadsBuildMetadata(t *testing.T) {
	clearEnvironment(t)
	original := [9]string{Version, Channel, Commit, CommitShort, WorkflowRun, Source, BuiltAt, ImageRef, ImageDigest}
	t.Cleanup(func() {
		Version, Channel, Commit, CommitShort, WorkflowRun = original[0], original[1], original[2], original[3], original[4]
		Source, BuiltAt, ImageRef, ImageDigest = original[5], original[6], original[7], original[8]
	})

	Version = "v1.2.3"
	Channel = "release"
	Commit = "0123456789abcdef"
	CommitShort = ""
	WorkflowRun = "987654321"
	Source = "https://github.com/abdul-mohsen/ifritah-go"
	BuiltAt = "2026-09-07T10:00:00Z"
	ImageRef = "docker.io/example/ifritah-api:v1.2.3"
	ImageDigest = "sha256:abc"

	got := Current()
	if got.Version != "v1.2.3" || got.Channel != "release" {
		t.Fatalf("identity version/channel = %#v", got)
	}
	if got.Commit != "0123456789abcdef" || got.CommitShort != "0123456" {
		t.Fatalf("identity commit fields = %#v", got)
	}
	if got.WorkflowRun != "987654321" || got.Source == "" || got.BuiltAt == "" {
		t.Fatalf("identity provenance fields = %#v", got)
	}
	if got.ImageRef == "" || got.ImageDigest == "" {
		t.Fatalf("identity image fields = %#v", got)
	}
}

func TestHandlerReturnsIdentityJSON(t *testing.T) {
	clearEnvironment(t)
	originalVersion, originalChannel, originalCommit := Version, Channel, Commit
	t.Cleanup(func() {
		Version, Channel, Commit = originalVersion, originalChannel, originalCommit
	})
	Version, Channel, Commit = "v0.0.1", "dev", "deadbeef1234567"

	request := httptest.NewRequest(http.MethodGet, "/version", nil)
	response := httptest.NewRecorder()
	Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q", got)
	}
	if response.Body.String() == "" {
		t.Fatal("version response is empty")
	}
	for _, expected := range []string{`"version":"v0.0.1"`, `"channel":"dev"`, `"commit":"deadbeef1234567"`, `"commit_short":"deadbee"`} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("response %q does not contain %q", response.Body.String(), expected)
		}
	}
}

func clearEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"APP_VERSION", "APP_CHANNEL", "APP_COMMIT", "APP_COMMIT_SHORT",
		"APP_WORKFLOW_RUN", "APP_SOURCE", "APP_CREATED", "APP_IMAGE_REF",
		"APP_IMAGE_DIGEST",
	} {
		t.Setenv(name, "")
	}
}
