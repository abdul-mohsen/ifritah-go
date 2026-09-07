package buildinfo

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// These values are replaced by the Docker build with -ldflags. The defaults
// keep local `go run` builds useful without requiring CI-only metadata.
var (
	Version     = "dev"
	Channel     = "dev"
	Commit      = "unknown"
	CommitShort = ""
	WorkflowRun = ""
	WorkflowURL = ""
	Source      = ""
	BuiltAt     = ""
	ImageRef    = ""
	ImageDigest = ""
)

type Identity struct {
	Version     string `json:"version"`
	Channel     string `json:"channel"`
	Commit      string `json:"commit"`
	CommitShort string `json:"short_commit,omitempty"`
	WorkflowRun string `json:"workflow_run,omitempty"`
	WorkflowURL string `json:"workflow_run_url,omitempty"`
	Source      string `json:"source,omitempty"`
	BuiltAt     string `json:"built_at,omitempty"`
	ImageRef    string `json:"image_ref,omitempty"`
	ImageDigest string `json:"digest,omitempty"`
}

func Current() Identity {
	commit := valueFrom(Commit, "APP_IMAGE_COMMIT", "APP_COMMIT")
	commitShort := valueFrom(CommitShort, "APP_IMAGE_COMMIT_SHORT", "APP_COMMIT_SHORT")
	if commitShort == "" {
		commitShort = shortCommit(commit)
	}

	return Identity{
		Version:     localVersion(),
		Channel:     valueFrom(Channel, "APP_IMAGE_CHANNEL", "APP_BUILD_CHANNEL", "APP_CHANNEL"),
		Commit:      commit,
		CommitShort: commitShort,
		WorkflowRun: valueFrom(WorkflowRun, "APP_WORKFLOW_RUN_ID", "APP_BUILD_WORKFLOW_RUN", "APP_WORKFLOW_RUN"),
		WorkflowURL: valueFrom(WorkflowURL, "APP_WORKFLOW_RUN_URL", "APP_BUILD_WORKFLOW_URL", "APP_WORKFLOW_URL"),
		Source:      valueFrom(Source, "APP_BUILD_SOURCE", "APP_SOURCE"),
		BuiltAt:     valueFrom(BuiltAt, "APP_BUILT_AT", "APP_BUILD_AT", "APP_CREATED"),
		ImageRef:    value(ImageRef, "APP_IMAGE_REF"),
		ImageDigest: value(ImageDigest, "APP_IMAGE_DIGEST"),
	}
}

func localVersion() string {
	version := valueFrom(Version, "APP_IMAGE_VERSION", "APP_VERSION")
	if version != "" && version != "dev" {
		return version
	}

	for _, path := range []string{"VERSION", filepath.Join("..", "..", "VERSION"), "/app/VERSION"} {
		data, err := os.ReadFile(path)
		if err == nil {
			if fileVersion := strings.TrimSpace(string(data)); fileVersion != "" {
				return fileVersion
			}
		}
	}
	return version
}

func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Current())
	}
}

func value(defaultValue, environmentName string) string {
	if environmentValue := strings.TrimSpace(os.Getenv(environmentName)); environmentValue != "" {
		return environmentValue
	}
	return defaultValue
}

func valueFrom(defaultValue string, environmentNames ...string) string {
	for _, environmentName := range environmentNames {
		if environmentValue := strings.TrimSpace(os.Getenv(environmentName)); environmentValue != "" {
			return environmentValue
		}
	}
	return defaultValue
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
