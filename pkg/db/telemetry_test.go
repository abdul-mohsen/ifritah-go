package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestRuntimeDatabaseOperationsCreateBoundedDependencySpans(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(previous)
	})

	database := sql.OpenDB(newTracingConnector(fakeConnector{}))
	defer database.Close()

	rows, err := database.QueryContext(context.Background(), "SELECT password FROM users WHERE token = ?", "secret")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close rows: %v", err)
	}
	if _, err := database.ExecContext(context.Background(), "UPDATE users SET password = ?", "secret"); err != nil {
		t.Fatalf("exec: %v", err)
	}
	transaction, err := database.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatalf("rollback transaction: %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) != 3 {
		t.Fatalf("database spans = %d, want 3: %#v", len(spans), spans)
	}
	seen := make(map[string]tracetest.SpanStub, len(spans))
	for _, span := range spans {
		seen[span.Name] = span
		if span.SpanKind != trace.SpanKindClient {
			t.Fatalf("span %q kind = %s, want client", span.Name, span.SpanKind)
		}
		for _, attr := range span.Attributes {
			if strings.Contains(attr.Value.AsString(), "password") ||
				strings.Contains(attr.Value.AsString(), "secret") {
				t.Fatalf("database value leaked in %q attributes: %#v", span.Name, span.Attributes)
			}
		}
	}
	for _, operation := range []string{"query", "exec", "transaction"} {
		span, ok := seen["mysql."+operation]
		if !ok {
			t.Fatalf("missing mysql.%s span: %#v", operation, seen)
		}
		attributes := make(map[string]string)
		for _, attr := range span.Attributes {
			attributes[string(attr.Key)] = attr.Value.AsString()
		}
		if attributes["db.system"] != "mysql" ||
			attributes["db.operation.name"] != operation {
			t.Fatalf("mysql.%s attributes = %#v", operation, attributes)
		}
	}
}

func TestRuntimeDatabaseErrorRecordsOnlyBoundedErrorType(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(previous)
	})

	database := sql.OpenDB(newTracingConnector(fakeConnector{
		execError: errors.New("UPDATE users SET password = 'secret'"),
	}))
	defer database.Close()

	if _, err := database.ExecContext(context.Background(), "UPDATE users SET password = ?", "secret"); err == nil {
		t.Fatal("exec succeeded, want error")
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("database spans = %d, want 1", len(spans))
	}
	if strings.Contains(spans[0].Name, "password") || strings.Contains(spans[0].Name, "secret") {
		t.Fatalf("database query text leaked in span name: %q", spans[0].Name)
	}
	if len(spans[0].Events) != 1 || spans[0].Events[0].Name != "exception" {
		t.Fatalf("database span events = %#v", spans[0].Events)
	}
	for _, attr := range spans[0].Events[0].Attributes {
		if strings.Contains(attr.Value.AsString(), "password") ||
			strings.Contains(attr.Value.AsString(), "secret") {
			t.Fatalf("database error text leaked: %#v", spans[0].Events[0].Attributes)
		}
	}
}

type fakeConnector struct {
	execError error
}

func (c fakeConnector) Connect(context.Context) (driver.Conn, error) {
	return fakeConn{execError: c.execError}, nil
}

func (fakeConnector) Driver() driver.Driver {
	return fakeDriver{}
}

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) {
	return fakeConn{}, nil
}

type fakeConn struct {
	execError error
}

func (c fakeConn) Prepare(string) (driver.Stmt, error) {
	return fakeStmt{}, nil
}

func (fakeConn) Close() error {
	return nil
}

func (fakeConn) Begin() (driver.Tx, error) {
	return fakeTx{}, nil
}

func (c fakeConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	if c.execError != nil {
		return nil, c.execError
	}
	return driver.RowsAffected(1), nil
}

func (fakeConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return fakeRows{}, nil
}

func (fakeConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return fakeTx{}, nil
}

type fakeStmt struct{}

func (fakeStmt) Close() error {
	return nil
}

func (fakeStmt) NumInput() int {
	return -1
}

func (fakeStmt) Exec([]driver.Value) (driver.Result, error) {
	return driver.RowsAffected(1), nil
}

func (fakeStmt) Query([]driver.Value) (driver.Rows, error) {
	return fakeRows{}, nil
}

type fakeTx struct{}

func (fakeTx) Commit() error {
	return nil
}

func (fakeTx) Rollback() error {
	return nil
}

type fakeRows struct{}

func (fakeRows) Columns() []string {
	return []string{"value"}
}

func (fakeRows) Close() error {
	return nil
}

func (fakeRows) Next([]driver.Value) error {
	return io.EOF
}

var _ driver.Driver = fakeDriver{}
var _ driver.Connector = fakeConnector{}
var _ driver.Conn = fakeConn{}
var _ driver.Stmt = fakeStmt{}
var _ driver.Tx = fakeTx{}
var _ driver.Rows = fakeRows{}
