package goosemigrations

import (
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var migrationFileName = regexp.MustCompile(`^(\d{5})_[a-z0-9_]+\.sql$`)

func TestMigrationFiles(t *testing.T) {
	entries, err := fs.ReadDir(FS, ".")
	if err != nil {
		t.Fatal(err)
	}

	var versions []int64
	for _, entry := range entries {
		name := entry.Name()
		match := migrationFileName.FindStringSubmatch(name)
		if match == nil {
			t.Errorf("%s: name must be NNNNN_snake_case.sql", name)
			continue
		}
		version, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		versions = append(versions, version)

		body, err := fs.ReadFile(FS, name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		if !strings.Contains(text, "-- +goose Up") {
			t.Errorf("%s: missing -- +goose Up", name)
		}
		if version > BaselineVersion && !hasDownStatements(text) {
			t.Errorf("%s: every migration after the baseline needs a -- +goose Down with statements", name)
		}
	}

	if len(versions) == 0 {
		t.Fatal("no goose migrations are embedded")
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })
	for i, version := range versions {
		if version != int64(i)+BaselineVersion {
			t.Fatalf("versions must be unique and contiguous from %d; got %v", BaselineVersion, versions)
		}
	}
}

// hasDownStatements reports whether the -- +goose Down section contains at least one SQL line.
func hasDownStatements(text string) bool {
	_, down, found := strings.Cut(text, "-- +goose Down")
	if !found {
		return false
	}
	for _, line := range strings.Split(down, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "--") {
			return true
		}
	}
	return false
}
