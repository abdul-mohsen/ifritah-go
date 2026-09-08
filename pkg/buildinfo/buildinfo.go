package buildinfo

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// These values are replaced by the Docker build with -ldflags. The defaults
// keep local `go run` builds useful without requiring CI-only metadata.
var (
	Version       = "dev"
	Channel       = "dev"
	Commit        = "unknown"
	CommitShort   = ""
	ImageTag      = ""
	ImageRef      = ""
	ImageDigest   = ""
	WorkflowRunID = ""
	WorkflowRun   = ""
	WorkflowURL   = ""
	Source        = ""
	BuiltAt       = ""
)

type Identity struct {
	Version       string `json:"version"`
	Tag           string `json:"tag,omitempty"`
	Ref           string `json:"ref,omitempty"`
	ImageRef      string `json:"image_ref,omitempty"`
	Digest        string `json:"digest,omitempty"`
	Channel       string `json:"channel"`
	Commit        string `json:"commit"`
	CommitShort   string `json:"short_commit,omitempty"`
	WorkflowRunID string `json:"workflow_run_id,omitempty"`
	WorkflowRun   string `json:"workflow_run,omitempty"`
	WorkflowURL   string `json:"workflow_run_url,omitempty"`
	Source        string `json:"source,omitempty"`
	BuiltAt       string `json:"built_at,omitempty"`
}

func Current() Identity {
	commit := valueFrom(Commit, "APP_IMAGE_COMMIT", "APP_COMMIT")
	commitShort := valueFrom(CommitShort, "APP_IMAGE_COMMIT_SHORT", "APP_COMMIT_SHORT")
	if commitShort == "" {
		commitShort = shortCommit(commit)
	}
	ref := valueFrom(ImageRef, "APP_IMAGE_REF", "APP_REF")
	tag := valueFrom(ImageTag, "APP_IMAGE_TAG", "APP_TAG")
	if tag == "" {
		tag = tagFromRef(ref)
	}
	workflowRunID := valueFrom(WorkflowRunID, "APP_WORKFLOW_RUN_ID", "APP_BUILD_WORKFLOW_RUN", "APP_WORKFLOW_RUN")
	if workflowRunID == "" {
		workflowRunID = WorkflowRun
	}

	return Identity{
		Version:       localVersion(),
		Tag:           tag,
		Ref:           ref,
		ImageRef:      ref,
		Digest:        valueFrom(ImageDigest, "APP_IMAGE_DIGEST", "APP_DIGEST"),
		Channel:       valueFrom(Channel, "APP_IMAGE_CHANNEL", "APP_BUILD_CHANNEL", "APP_CHANNEL"),
		Commit:        commit,
		CommitShort:   commitShort,
		WorkflowRunID: workflowRunID,
		WorkflowRun:   workflowRunID,
		WorkflowURL:   valueFrom(WorkflowURL, "APP_WORKFLOW_RUN_URL", "APP_BUILD_WORKFLOW_URL", "APP_WORKFLOW_URL"),
		Source:        valueFrom(Source, "APP_BUILD_SOURCE", "APP_SOURCE"),
		BuiltAt:       valueFrom(BuiltAt, "APP_BUILT_AT", "APP_BUILD_AT", "APP_CREATED"),
	}
}

var semanticVersionPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

func localVersion() string {
	version := valueFrom(Version, "APP_IMAGE_VERSION", "APP_VERSION")
	if isSemanticVersion(version) {
		return version
	}

	for _, path := range []string{"VERSION", filepath.Join("..", "..", "VERSION"), "/app/VERSION"} {
		data, err := os.ReadFile(path)
		if err == nil {
			if fileVersion := strings.TrimSpace(string(data)); fileVersion != "" {
				if isSemanticVersion(fileVersion) {
					return fileVersion
				}
			}
		}
	}
	if version == "" || version == "v0.0.0" {
		return "dev"
	}
	return version
}

func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Current())
	}
}

func valueFrom(defaultValue string, environmentNames ...string) string {
	for _, environmentName := range environmentNames {
		if environmentValue := strings.TrimSpace(os.Getenv(environmentName)); environmentValue != "" {
			return environmentValue
		}
	}
	return defaultValue
}

func isSemanticVersion(version string) bool {
	return version != "v0.0.0" && semanticVersionPattern.MatchString(version)
}

func shortCommit(commit string) string {
	if commit == "" || commit == "unknown" {
		return ""
	}
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}

func tagFromRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	ref = strings.SplitN(ref, "@", 2)[0]
	lastSlash := strings.LastIndex(ref, "/")
	lastColon := strings.LastIndex(ref, ":")
	if lastColon > lastSlash && lastColon+1 < len(ref) {
		return ref[lastColon+1:]
	}
	return ""
}
