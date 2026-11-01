package goosemigrations

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

// TestSubmissionStateContract checks 00003 on a database that already has submissions. Each
// existing submission gets the outcome that its numeric status implies. The new columns and
// tables enforce the outcome states, the attempt numbers and the claim keys. It needs a MySQL
// server, like TestBaselineMatchesLegacySchema.
func TestSubmissionStateContract(t *testing.T) {
	dsn := os.Getenv("GOOSE_PARITY_DSN")
	if dsn == "" {
		t.Skip("GOOSE_PARITY_DSN is not set")
	}

	const name = "goose_submission_state"
	admin := openTestDB(t, dsn, "")
	defer admin.Close()
	mustExec(t, admin, "DROP DATABASE IF EXISTS `"+name+"`")
	mustExec(t, admin, "CREATE DATABASE `"+name+"` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
	defer mustExec(t, admin, "DROP DATABASE IF EXISTS `"+name+"`")

	db := openTestDB(t, dsn, name)
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectMySQL, db, FS)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// One submission per numeric status, created before 00003 exists.
	if _, err := provider.UpTo(ctx, 2); err != nil {
		t.Fatalf("goose up to version 2: %v", err)
	}
	wantOutcome := map[int]string{0: "pending", 1: "pending", 2: "accepted", 3: "rejected", 4: "accepted_with_warning"}
	submissionID := map[int]int64{}
	for status := range wantOutcome {
		res, err := db.Exec("INSERT INTO zatca_submission (bill_id, status) VALUES (?, ?)", insertTestBill(t, db), status)
		if err != nil {
			t.Fatalf("insert submission with status %d: %v", status, err)
		}
		if submissionID[status], err = res.LastInsertId(); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("goose up to version 3: %v", err)
	}
	for status, want := range wantOutcome {
		var got string
		if err := db.QueryRow("SELECT outcome_state FROM zatca_submission WHERE id = ?", submissionID[status]).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("status %d: outcome_state = %q, want %q", status, got, want)
		}
	}

	sub := submissionID[2]
	_, err = db.Exec("UPDATE zatca_submission SET outcome_state = 'done' WHERE id = ?", sub)
	requireDBError(t, "unknown outcome state", "chk_zatca_submission_outcome_state", err)

	_, err = db.Exec("INSERT INTO zatca_submission_attempt (submission_id, attempt_number, attempt_state, started_at) VALUES (?, 1, 'pending', NOW())", sub)
	if err != nil {
		t.Fatalf("first attempt: %v", err)
	}
	_, err = db.Exec("INSERT INTO zatca_submission_attempt (submission_id, attempt_number, attempt_state, started_at) VALUES (?, 1, 'pending', NOW())", sub)
	requireDBError(t, "duplicate attempt number", "Duplicate entry", err)
	_, err = db.Exec("INSERT INTO zatca_submission_attempt (submission_id, attempt_number, attempt_state, started_at) VALUES (?, 2, 'running', NOW())", sub)
	requireDBError(t, "unknown attempt state", "chk_zatca_attempt_state", err)

	var attempt int64
	if err := db.QueryRow("SELECT id FROM zatca_submission_attempt WHERE submission_id = ? AND attempt_number = 1", sub).Scan(&attempt); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO zatca_submission_outcome (submission_id, attempt_id, outcome_state, observed_at) VALUES (?, ?, 'accepted', NOW())", sub, attempt); err != nil {
		t.Fatalf("outcome: %v", err)
	}
	sha := strings.Repeat("a", 64)
	if _, err := db.Exec("INSERT INTO zatca_submission_artifact (submission_id, attempt_id, artifact_kind, artifact_path, sha256) VALUES (?, ?, 'request_xml', 'requests/2.xml', ?)", sub, attempt, sha); err != nil {
		t.Fatalf("artifact: %v", err)
	}
	_, err = db.Exec("INSERT INTO zatca_submission_artifact (submission_id, artifact_kind) VALUES (?, 'screenshot')", sub)
	requireDBError(t, "unknown artifact kind", "chk_zatca_artifact_kind", err)
	_, err = db.Exec("INSERT INTO zatca_submission_artifact (submission_id, artifact_kind, sha256) VALUES (?, 'other', ?)", sub, strings.ToUpper(sha))
	requireDBError(t, "uppercase sha256", "chk_zatca_artifact_sha256", err)

	const requestID = "0b9c1f3e-3c2a-4a8e-9d4b-2f6a1c7e8d90"
	if _, err := db.Exec("INSERT INTO zatca_consumer_claim (pos_id, bill_request_id, event_id, egs_sequence_id, fingerprint) VALUES ('pos-1', ?, 'evt-1', 'egs-1', 'fp-1')", requestID); err != nil {
		t.Fatalf("claim: %v", err)
	}
	_, err = db.Exec("INSERT INTO zatca_consumer_claim (pos_id, bill_request_id, event_id, egs_sequence_id, fingerprint) VALUES ('pos-2', ?, 'evt-1', 'egs-2', 'fp-2')", requestID)
	requireDBError(t, "duplicate event id", "Duplicate entry", err)
	_, err = db.Exec("INSERT INTO zatca_consumer_claim (pos_id, bill_request_id, event_id, egs_sequence_id, fingerprint) VALUES ('pos-1', ?, 'evt-2', 'egs-1', 'fp-3')", requestID)
	requireDBError(t, "duplicate request on the same POS", "Duplicate entry", err)
	_, err = db.Exec("INSERT INTO zatca_consumer_claim (pos_id, bill_request_id, event_id, egs_sequence_id, fingerprint, state) VALUES ('pos-3', ?, 'evt-3', 'egs-1', 'fp-4', 'running')", requestID)
	requireDBError(t, "unknown claim state", "chk_zatca_claim_state", err)
}

// TestAppendOnlyTablesHaveNoUpdateOrDelete guards zatca_submission_outcome and
// zatca_submission_artifact. Their rows are never changed after insert. The database cannot
// enforce this without a trigger, and sqlc cannot parse triggers, so every sqlc query is checked.
func TestAppendOnlyTablesHaveNoUpdateOrDelete(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "queries", "*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no sqlc query files found")
	}
	appendOnly := regexp.MustCompile(`(?i)\bzatca_submission_(outcome|artifact)\b`)
	write := regexp.MustCompile(`(?i)\b(update|delete)\b`)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		// Every sqlc query starts with a "-- name:" line.
		for _, query := range strings.Split(string(data), "-- name:")[1:] {
			if appendOnly.MatchString(query) && write.MatchString(query) {
				t.Fatalf("%s: a query writes an append-only table:\n%s", filepath.Base(file), strings.TrimSpace(query))
			}
		}
	}
}

// insertTestBill creates a bill with only the columns that have no default, and returns its id.
func insertTestBill(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	res, err := db.Exec("INSERT INTO bill (discount, store_id, merchant_id, maintenance_cost) VALUES (0, 1, 1, 0)")
	if err != nil {
		t.Fatalf("insert bill: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// requireDBError fails unless err is set and its message contains want.
func requireDBError(t *testing.T, what, want string, err error) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("%s was not rejected with %q: %v", what, want, err)
	}
}
