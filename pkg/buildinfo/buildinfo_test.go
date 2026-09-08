package buildinfo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCurrentLoadsBuildMetadata(t *testing.T) {
	clearEnvironment(t)
	restoreBuildVars(t)

	Version = "v1.2.3"
	Channel = "release"
	Commit = "0123456789abcdef"
	CommitShort = ""
	ImageTag = "v1.2.3"
	ImageRef = "docker.io/example/ifritah-api:v1.2.3"
	ImageDigest = "sha256:abc"
	WorkflowRunID = "987654321"
	WorkflowURL = "https://github.com/abdul-mohsen/ifritah-go/actions/runs/987654321"
	Source = "https://github.com/abdul-mohsen/ifritah-go"
	BuiltAt = "2026-09-07T10:00:00Z"

	got := Current()
	if got.Version != "v1.2.3" || got.Channel != "release" {
		t.Fatalf("identity version/channel = %#v", got)
	}
	if got.Commit != "0123456789abcdef" || got.CommitShort != "0123456" {
		t.Fatalf("identity commit fields = %#v", got)
	}
	if got.WorkflowRunID != "987654321" || got.WorkflowRun != "987654321" ||
		got.WorkflowURL == "" || got.Source == "" || got.BuiltAt == "" {
		t.Fatalf("identity provenance fields = %#v", got)
	}
	if got.Tag != "v1.2.3" || got.Ref != "docker.io/example/ifritah-api:v1.2.3" ||
		got.ImageRef != got.Ref || got.Digest != "sha256:abc" {
		t.Fatalf("identity image fields = %#v", got)
	}
}

func TestCurrentPrefersCanonicalRuntimeEnvironment(t *testing.T) {
	clearEnvironment(t)
	restoreBuildVars(t)

	Version, Channel, Commit = "v1.2.3", "dev", "linked-commit"
	t.Setenv("APP_BUILD_CHANNEL", "release")
	t.Setenv("APP_IMAGE_VERSION", "v9.8.7")
	t.Setenv("APP_IMAGE_COMMIT", "canonical-commit")
	t.Setenv("APP_IMAGE_CHANNEL", "canonical-channel")
	t.Setenv("APP_IMAGE_TAG", "dev")
	t.Setenv("APP_WORKFLOW_RUN_ID", "456")
	t.Setenv("APP_WORKFLOW_RUN_URL", "https://example.test/actions/runs/456")
	t.Setenv("APP_BUILD_WORKFLOW_RUN", "123")
	t.Setenv("APP_BUILD_WORKFLOW_URL", "https://example.test/runs/123")
	t.Setenv("APP_BUILT_AT", "2026-09-07T10:00:00Z")
	t.Setenv("APP_IMAGE_REF", "registry.example/api:dev")
	t.Setenv("APP_IMAGE_DIGEST", "sha256:abc")

	got := Current()
	if got.Version != "v9.8.7" || got.Commit != "canonical-commit" ||
		got.Channel != "canonical-channel" || got.Tag != "dev" ||
		got.WorkflowRunID != "456" || got.WorkflowRun != "456" || got.WorkflowURL == "" {
		t.Fatalf("canonical runtime metadata = %#v", got)
	}
	if got.BuiltAt == "" || got.Ref == "" || got.ImageRef == "" || got.Digest == "" {
		t.Fatalf("canonical runtime identity = %#v", got)
	}
}

func TestCurrentDerivesImageTagFromRef(t *testing.T) {
	clearEnvironment(t)
	restoreBuildVars(t)

	Version = "v1.2.3"
	ImageRef = "registry.example:5000/team/ifritah-api:dev"

	got := Current()
	if got.Tag != "dev" || got.Ref != ImageRef {
		t.Fatalf("derived tag/ref = %#v", got)
	}
}

func TestCurrentDoesNotUseV000Fallback(t *testing.T) {
	clearEnvironment(t)
	restoreBuildVars(t)

	Version = "v0.0.0"
	t.Setenv("APP_IMAGE_VERSION", "v0.0.0")

	got := Current()
	if got.Version == "v0.0.0" {
		t.Fatalf("version used forbidden fallback: %#v", got)
	}
}

func TestHandlerReturnsIdentityJSON(t *testing.T) {
	clearEnvironment(t)
	restoreBuildVars(t)
	Version, Channel, Commit = "v0.0.1", "dev", "deadbeef1234567"
	ImageTag, ImageRef, ImageDigest = "dev", "docker.io/example/ifritah-api:dev", "sha256:abc"
	WorkflowRunID, WorkflowURL = "123456789", "https://example.test/actions/runs/123456789"

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
	if _, ok := fields["workflow_run_id"]; !ok {
		t.Fatal("version response is missing canonical workflow_run_id field")
	}
	if _, ok := fields["tag"]; !ok {
		t.Fatal("version response is missing canonical tag field")
	}
	if _, ok := fields["ref"]; !ok {
		t.Fatal("version response is missing canonical ref field")
	}
	if _, ok := fields["commit_short"]; ok {
		t.Fatal("version response contains legacy commit_short field")
	}
}

func restoreBuildVars(t *testing.T) {
	t.Helper()
	original := struct {
		version       string
		channel       string
		commit        string
		commitShort   string
		imageTag      string
		imageRef      string
		imageDigest   string
		workflowRunID string
		workflowRun   string
		workflowURL   string
		source        string
		builtAt       string
	}{
		Version,
		Channel,
		Commit,
		CommitShort,
		ImageTag,
		ImageRef,
		ImageDigest,
		WorkflowRunID,
		WorkflowRun,
		WorkflowURL,
		Source,
		BuiltAt,
	}
	t.Cleanup(func() {
		Version = original.version
		Channel = original.channel
		Commit = original.commit
		CommitShort = original.commitShort
		ImageTag = original.imageTag
		ImageRef = original.imageRef
		ImageDigest = original.imageDigest
		WorkflowRunID = original.workflowRunID
		WorkflowRun = original.workflowRun
		WorkflowURL = original.workflowURL
		Source = original.source
		BuiltAt = original.builtAt
	})
}

func clearEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"APP_VERSION", "APP_CHANNEL", "APP_COMMIT", "APP_COMMIT_SHORT",
		"APP_IMAGE_VERSION", "APP_IMAGE_COMMIT", "APP_IMAGE_COMMIT_SHORT", "APP_IMAGE_TAG",
		"APP_IMAGE_CHANNEL", "APP_WORKFLOW_RUN_ID", "APP_WORKFLOW_RUN_URL",
		"APP_BUILD_CHANNEL", "APP_BUILD_SOURCE", "APP_BUILD_WORKFLOW_RUN",
		"APP_BUILD_WORKFLOW_URL", "APP_BUILT_AT", "APP_BUILD_AT",
		"APP_WORKFLOW_RUN", "APP_WORKFLOW_URL", "APP_SOURCE", "APP_CREATED", "APP_TAG",
		"APP_IMAGE_REF", "APP_IMAGE_DIGEST", "APP_REF", "APP_DIGEST",
	} {
		t.Setenv(name, "")
	}
}
