package buildinfo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCurrentLoadsBuildMetadata(t *testing.T) {
	clearEnvironment(t)
	original := [10]string{Version, Channel, Commit, CommitShort, WorkflowRun, WorkflowURL, Source, BuiltAt, ImageRef, ImageDigest}
	t.Cleanup(func() {
		Version, Channel, Commit, CommitShort, WorkflowRun = original[0], original[1], original[2], original[3], original[4]
		WorkflowURL, Source, BuiltAt, ImageRef, ImageDigest = original[5], original[6], original[7], original[8], original[9]
	})

	Version = "v1.2.3"
	Channel = "release"
	Commit = "0123456789abcdef"
	CommitShort = ""
	WorkflowRun = "987654321"
	WorkflowURL = "https://github.com/abdul-mohsen/ifritah-go/actions/runs/987654321"
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
	if got.WorkflowRun != "987654321" || got.WorkflowURL == "" || got.Source == "" || got.BuiltAt == "" {
		t.Fatalf("identity provenance fields = %#v", got)
	}
	if got.ImageRef == "" || got.ImageDigest == "" {
		t.Fatalf("identity image fields = %#v", got)
	}
}

func TestCurrentPrefersCanonicalRuntimeEnvironment(t *testing.T) {
	clearEnvironment(t)
	original := [10]string{Version, Channel, Commit, CommitShort, WorkflowRun, WorkflowURL, Source, BuiltAt, ImageRef, ImageDigest}
	t.Cleanup(func() {
		Version, Channel, Commit, CommitShort, WorkflowRun = original[0], original[1], original[2], original[3], original[4]
		WorkflowURL, Source, BuiltAt, ImageRef, ImageDigest = original[5], original[6], original[7], original[8], original[9]
	})

	Version, Channel, Commit = "v1.2.3", "dev", "linked-commit"
	t.Setenv("APP_BUILD_CHANNEL", "release")
	t.Setenv("APP_IMAGE_VERSION", "v9.8.7")
	t.Setenv("APP_IMAGE_COMMIT", "canonical-commit")
	t.Setenv("APP_IMAGE_CHANNEL", "canonical-channel")
	t.Setenv("APP_WORKFLOW_RUN_ID", "456")
	t.Setenv("APP_WORKFLOW_RUN_URL", "https://example.test/actions/runs/456")
	t.Setenv("APP_BUILD_WORKFLOW_RUN", "123")
	t.Setenv("APP_BUILD_WORKFLOW_URL", "https://example.test/runs/123")
	t.Setenv("APP_BUILT_AT", "2026-09-07T10:00:00Z")
	t.Setenv("APP_IMAGE_REF", "registry.example/api:v1.2.3")
	t.Setenv("APP_IMAGE_DIGEST", "sha256:abc")

	got := Current()
	if got.Version != "v9.8.7" || got.Commit != "canonical-commit" ||
		got.Channel != "canonical-channel" || got.WorkflowRun != "456" || got.WorkflowURL == "" {
		t.Fatalf("canonical runtime metadata = %#v", got)
	}
	if got.BuiltAt == "" || got.ImageRef == "" || got.ImageDigest == "" {
		t.Fatalf("canonical runtime identity = %#v", got)
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
	var got Identity
	if err := json.NewDecoder(response.Body).Decode(&got); err != nil {
		t.Fatalf("decode version response: %v", err)
	}
	if got.Version != "v0.0.1" || got.Channel != "dev" || got.Commit != "deadbeef1234567" || got.CommitShort != "deadbee" {
		t.Fatalf("version response = %#v", got)
	}

	response = httptest.NewRecorder()
	Handler().ServeHTTP(response, request)
	var fields map[string]json.RawMessage
	if err := json.NewDecoder(response.Body).Decode(&fields); err != nil {
		t.Fatalf("decode version fields: %v", err)
	}
	if _, ok := fields["short_commit"]; !ok {
		t.Fatal("version response is missing canonical short_commit field")
	}
	if _, ok := fields["commit_short"]; ok {
		t.Fatal("version response contains legacy commit_short field")
	}
}

func clearEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"APP_VERSION", "APP_CHANNEL", "APP_COMMIT", "APP_COMMIT_SHORT",
		"APP_IMAGE_VERSION", "APP_IMAGE_COMMIT", "APP_IMAGE_COMMIT_SHORT",
		"APP_IMAGE_CHANNEL", "APP_WORKFLOW_RUN_ID", "APP_WORKFLOW_RUN_URL",
		"APP_BUILD_CHANNEL", "APP_BUILD_SOURCE", "APP_BUILD_WORKFLOW_RUN",
		"APP_BUILD_WORKFLOW_URL", "APP_BUILT_AT", "APP_BUILD_AT",
		"APP_WORKFLOW_RUN", "APP_WORKFLOW_URL", "APP_SOURCE", "APP_CREATED",
		"APP_IMAGE_REF", "APP_IMAGE_DIGEST",
	} {
		t.Setenv(name, "")
	}
}
