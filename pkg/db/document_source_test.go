package db

import (
	"strings"
	"testing"
)

func TestZatcaDocumentSourceCoversBillCreditAndDebit(t *testing.T) {
	schema := readDatabaseSource(t, "schema", "schema.sql")
	queries := readDatabaseSource(t, "queries", "document_source.sql")
	egsDocument := tableSource(t, schema, "zatca_egs_document")
	outbox := tableSource(t, schema, "outbox")
	creditNote := tableSource(t, schema, "credit_note")
	debitNote := tableSource(t, schema, "debit_note")

	requireSourceContains(t, schema, "CREATE VIEW `zatca_document_source`")
	for _, documentType := range []string{"'bill'", "'credit'", "'debit'"} {
		requireSourceContains(t, schema, documentType+" AS document_type")
	}
	for _, column := range []string{
		"`original_document_type`",
		"`original_source_id`",
		"`original_invoice_uuid`",
	} {
		requireSourceContains(t, egsDocument, column)
		requireSourceContains(t, outbox, column)
	}
	for _, column := range []string{"`document_type`", "`source_id`", "`request_id`"} {
		requireSourceContains(t, outbox, column)
		requireSourceContains(t, egsDocument, column)
	}
	requireSourceContains(t, creditNote, "`request_id`")
	requireSourceContains(t, debitNote, "`request_id`")
	for _, queryName := range []string{
		"-- name: GetZatcaDocumentSource :one",
		"-- name: GetZatcaDocumentSourceByInvoiceUUID :one",
		"-- name: GetZatcaDocumentSourceByRequest :one",
	} {
		requireSourceContains(t, queries, queryName)
	}
}

func TestZatcaDocumentSourceOriginalReferences(t *testing.T) {
	schema := readDatabaseSource(t, "schema", "schema.sql")

	for _, documentType := range []string{"credit", "debit"} {
		t.Run(documentType, func(t *testing.T) {
			branch := strings.Index(schema, "'"+documentType+"' AS document_type")
			if branch < 0 {
				t.Fatalf("document source query does not define %s documents", documentType)
			}
			section := schema[branch:]
			requireSourceContains(t, section, "'bill' AS original_document_type")
			requireSourceContains(t, section, "original_source_id")
			requireSourceContains(t, section, "original_invoice_uuid")
		})
	}
}

func TestZatcaGenericSubmissionContract(t *testing.T) {
	schema := readDatabaseSource(t, "schema", "schema.sql")
	submission := tableSource(t, schema, "zatca_submission")
	outbox := tableSource(t, schema, "outbox")
	queries := readDatabaseSource(t, "queries", "zatca_submission.sql")
	outboxQueries := readDatabaseSource(t, "queries", "outbox.sql")
	monitor := readDatabaseSource(t, "queries", "zatca_monitor.sql")

	for _, column := range []string{
		"`document_type`",
		"`source_id`",
		"`request_id`",
		"`original_document_type`",
		"`original_source_id`",
		"`original_invoice_uuid`",
	} {
		requireSourceContains(t, submission, column)
	}
	if strings.Contains(submission, "`bill_id`") || strings.Contains(submission, "`bill_request_id`") {
		t.Fatal("submission schema must not retain bill-only source identity")
	}
	if strings.Contains(outbox, "`bill_request_id`") {
		t.Fatal("outbox schema must use generic request identity")
	}
	for _, queryName := range []string{
		"-- name: GetOutboxEventByRequestIdentity :one",
		"-- name: GetOutboxEventByDocument :one",
	} {
		requireSourceContains(t, outboxQueries, queryName)
	}
	for _, queryName := range []string{
		"-- name: UpsertZatcaSubmission :execresult",
		"-- name: RecordZatcaSubmissionOutcome :exec",
		"-- name: GetZatcaSubmissionByDocument :one",
		"-- name: GetZatcaSubmissionByRequest :one",
		"-- name: GetZatcaSubmissionByInvoiceUUID :one",
	} {
		requireSourceContains(t, queries, queryName)
	}
	if strings.Contains(queries, "bill_id") || strings.Contains(queries, "bill_request_id") {
		t.Fatal("submission SQL must use generic document/source/request identity")
	}
	if strings.Contains(monitor, "JOIN bill") || strings.Contains(monitor, "zs.bill_id") {
		t.Fatal("monitor SQL must not assume bill-only submissions")
	}
	requireSourceContains(t, monitor, "zatca_document_source")
	requireSourceContains(t, monitor, "document_type")
	requireSourceContains(t, monitor, "source_id")
}
