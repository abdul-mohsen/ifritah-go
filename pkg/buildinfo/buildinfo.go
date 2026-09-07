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
	Source      = ""
	BuiltAt     = ""
	ImageRef    = ""
	ImageDigest = ""
)

type Identity struct {
	Version     string `json:"version"`
	Channel     string `json:"channel"`
	Commit      string `json:"commit"`
	CommitShort string `json:"commit_short"`
	WorkflowRun string `json:"workflow_run,omitempty"`
	Source      string `json:"source,omitempty"`
	BuiltAt     string `json:"built_at,omitempty"`
	ImageRef    string `json:"image_ref,omitempty"`
	ImageDigest string `json:"image_digest,omitempty"`
}

func Current() Identity {
	commit := value(Commit, "APP_COMMIT")
	commitShort := value(CommitShort, "APP_COMMIT_SHORT")
	if commitShort == "" {
		commitShort = shortCommit(commit)
	}

	return Identity{
		Version:     localVersion(),
		Channel:     value(Channel, "APP_CHANNEL"),
		Commit:      commit,
		CommitShort: commitShort,
		WorkflowRun: value(WorkflowRun, "APP_WORKFLOW_RUN"),
		Source:      value(Source, "APP_SOURCE"),
		BuiltAt:     value(BuiltAt, "APP_CREATED"),
		ImageRef:    value(ImageRef, "APP_IMAGE_REF"),
		ImageDigest: value(ImageDigest, "APP_IMAGE_DIGEST"),
	}
}

func localVersion() string {
	version := value(Version, "APP_VERSION")
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

func shortCommit(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}
