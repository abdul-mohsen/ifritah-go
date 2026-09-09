package handlers

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestResolveUploadPathRejectsPathTraversal(t *testing.T) {
	for _, fileKey := range []string{
		"",
		".",
		"..",
		"../secret.txt",
		`..\secret.txt`,
		"nested/file.txt",
		`nested\file.txt`,
		"/tmp/secret.txt",
	} {
		t.Run(strings.ReplaceAll(fileKey, "/", "_"), func(t *testing.T) {
			if _, err := resolveUploadPath(fileKey); err == nil {
				t.Fatalf("resolveUploadPath(%q) accepted an unsafe key", fileKey)
			}
		})
	}
}

func TestResolveUploadPathKeepsFilesInsideUploadDirectory(t *testing.T) {
	got, err := resolveUploadPath("safe-file.pdf")
	if err != nil {
		t.Fatalf("resolveUploadPath returned error: %v", err)
	}

	uploadDir, err := filepath.Abs(FilesUploadDir)
	if err != nil {
		t.Fatalf("filepath.Abs(upload dir): %v", err)
	}
	relative, err := filepath.Rel(uploadDir, got)
	if err != nil {
		t.Fatalf("filepath.Rel: %v", err)
	}
	if relative != "safe-file.pdf" {
		t.Fatalf("resolved path is outside upload directory: %q", relative)
	}
}

func TestBuildCashVoucherListArgsUsesStaticSentinels(t *testing.T) {
	state := 1
	sequenceNumber := "12"
	cursorDate := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)

	args, err := buildCashVoucherListArgs(cashVoucherListRequest{
		Query:          "alice",
		VoucherType:    "receipt",
		State:          &state,
		SequenceNumber: &sequenceNumber,
	}, 7, &cursorDate, uint64Ptr(9), 25)
	if err != nil {
		t.Fatalf("buildCashVoucherListArgs returned error: %v", err)
	}

	want := []any{
		int64(7),
		"receipt", "receipt",
		1, 1,
		"alice", "%alice%", "%alice%", "%alice%",
		"12%", "12%",
		cursorDate, cursorDate, cursorDate, uint64(9),
		26,
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestBuildCashVoucherListArgsRejectsUnknownVoucherType(t *testing.T) {
	_, err := buildCashVoucherListArgs(cashVoucherListRequest{
		VoucherType: "unexpected",
	}, 7, nil, nil, 25)
	if err != errInvalidVoucherType {
		t.Fatalf("error = %v, want %v", err, errInvalidVoucherType)
	}
}

func uint64Ptr(value uint64) *uint64 {
	return &value
}
