package db

import (
	"context"
	"database/sql/driver"
	"sync"

	"ifritah/web-service-gin/pkg/telemetry"

	"go.opentelemetry.io/otel/trace"
)

const (
	databaseSystem      = "mysql"
	databaseQuery       = "query"
	databaseExec        = "exec"
	databaseTransaction = "transaction"
	databasePrepare     = "prepare"
	databasePing        = "ping"
)

type tracingConnector struct {
	connector driver.Connector
}

func newTracingConnector(connector driver.Connector) driver.Connector {
	return tracingConnector{connector: connector}
}

func (c tracingConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &tracingConn{Conn: conn}, nil
}

func (c tracingConnector) Driver() driver.Driver {
	return c.connector.Driver()
}

type tracingConn struct {
	driver.Conn
}

func (c *tracingConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (result driver.Result, err error) {
	spanCtx, span := startDatabaseSpan(ctx, databaseExec)
	defer func() {
		endDatabaseSpan(spanCtx, span, databaseExec, err)
	}()

	execer, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	return execer.ExecContext(ctx, query, args)
}

func (c *tracingConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (rows driver.Rows, err error) {
	spanCtx, span := startDatabaseSpan(ctx, databaseQuery)
	defer func() {
		endDatabaseSpan(spanCtx, span, databaseQuery, err)
	}()

	queryer, ok := c.Conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	return queryer.QueryContext(ctx, query, args)
}

func (c *tracingConn) Prepare(query string) (driver.Stmt, error) {
	stmt, err := c.Conn.Prepare(query)
	if err != nil {
		return nil, err
	}
	return &tracingStmt{Stmt: stmt}, nil
}

func (c *tracingConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	var (
		stmt driver.Stmt
		err  error
	)
	if preparer, ok := c.Conn.(driver.ConnPrepareContext); ok {
		stmt, err = preparer.PrepareContext(ctx, query)
	} else {
		stmt, err = c.Conn.Prepare(query)
	}
	if err != nil {
		return nil, err
	}
	return &tracingStmt{Stmt: stmt}, nil
}

func (c *tracingConn) Begin() (driver.Tx, error) {
	return c.begin(context.Background(), driver.TxOptions{})
}

func (c *tracingConn) BeginTx(ctx context.Context, options driver.TxOptions) (driver.Tx, error) {
	return c.begin(ctx, options)
}

func (c *tracingConn) begin(ctx context.Context, options driver.TxOptions) (driver.Tx, error) {
	spanCtx, span := startDatabaseSpan(ctx, databaseTransaction)

	var (
		tx  driver.Tx
		err error
	)
	if beginner, ok := c.Conn.(driver.ConnBeginTx); ok {
		tx, err = beginner.BeginTx(ctx, options)
	} else if options != (driver.TxOptions{}) {
		err = driver.ErrSkip
	} else {
		tx, err = c.Conn.Begin()
	}
	if err != nil {
		endDatabaseSpan(spanCtx, span, databaseTransaction, err)
		return nil, err
	}
	return &tracingTx{
		Tx:   tx,
		ctx:  spanCtx,
		span: span,
	}, nil
}

func (c *tracingConn) Ping(ctx context.Context) error {
	spanCtx, span := startDatabaseSpan(ctx, databasePing)

	pinger, ok := c.Conn.(driver.Pinger)
	if !ok {
		endDatabaseSpan(spanCtx, span, databasePing, driver.ErrSkip)
		return driver.ErrSkip
	}
	err := pinger.Ping(ctx)
	endDatabaseSpan(spanCtx, span, databasePing, err)
	return err
}

func (c *tracingConn) CheckNamedValue(value *driver.NamedValue) error {
	if checker, ok := c.Conn.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(value)
	}
	return driver.ErrSkip
}

func (c *tracingConn) ResetSession(ctx context.Context) error {
	if resetter, ok := c.Conn.(driver.SessionResetter); ok {
		return resetter.ResetSession(ctx)
	}
	return nil
}

func (c *tracingConn) IsValid() bool {
	if validator, ok := c.Conn.(driver.Validator); ok {
		return validator.IsValid()
	}
	return true
}

type tracingStmt struct {
	driver.Stmt
}

func (s *tracingStmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.exec(context.Background(), func() (driver.Result, error) {
		return s.Stmt.Exec(args)
	})
}

func (s *tracingStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if execer, ok := s.Stmt.(driver.StmtExecContext); ok {
		return s.exec(ctx, func() (driver.Result, error) {
			return execer.ExecContext(ctx, args)
		})
	}
	return nil, driver.ErrSkip
}

func (s *tracingStmt) exec(ctx context.Context, execute func() (driver.Result, error)) (driver.Result, error) {
	spanCtx, span := startDatabaseSpan(ctx, databaseExec)
	result, err := execute()
	endDatabaseSpan(spanCtx, span, databaseExec, err)
	return result, err
}

func (s *tracingStmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.query(context.Background(), func() (driver.Rows, error) {
		return s.Stmt.Query(args)
	})
}

func (s *tracingStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if queryer, ok := s.Stmt.(driver.StmtQueryContext); ok {
		return s.query(ctx, func() (driver.Rows, error) {
			return queryer.QueryContext(ctx, args)
		})
	}
	return nil, driver.ErrSkip
}

func (s *tracingStmt) query(ctx context.Context, query func() (driver.Rows, error)) (driver.Rows, error) {
	spanCtx, span := startDatabaseSpan(ctx, databaseQuery)
	rows, err := query()
	endDatabaseSpan(spanCtx, span, databaseQuery, err)
	return rows, err
}

func (s *tracingStmt) CheckNamedValue(value *driver.NamedValue) error {
	if checker, ok := s.Stmt.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(value)
	}
	return driver.ErrSkip
}

type tracingTx struct {
	driver.Tx
	ctx     context.Context
	span    trace.Span
	endOnce sync.Once
}

func (tx *tracingTx) Commit() error {
	err := tx.Tx.Commit()
	tx.finish("commit", err)
	return err
}

func (tx *tracingTx) Rollback() error {
	err := tx.Tx.Rollback()
	tx.finish("rollback", err)
	return err
}

func (tx *tracingTx) finish(operation string, err error) {
	tx.endOnce.Do(func() {
		if err != nil {
			telemetry.RecordError(tx.ctx, err, "mysql.transaction."+operation)
		}
		tx.span.End()
	})
}

func startDatabaseSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	spanCtx, span := telemetry.StartClientSpan(ctx, "mysql."+operation)
	telemetry.SetDatabaseAttributes(spanCtx, databaseSystem, operation)
	return spanCtx, span
}

func endDatabaseSpan(ctx context.Context, span trace.Span, operation string, err error) {
	if err != nil {
		telemetry.RecordError(ctx, err, "mysql."+operation)
	}
	span.End()
}

var (
	_ driver.Connector          = tracingConnector{}
	_ driver.ConnBeginTx        = (*tracingConn)(nil)
	_ driver.ConnPrepareContext = (*tracingConn)(nil)
	_ driver.ExecerContext      = (*tracingConn)(nil)
	_ driver.NamedValueChecker  = (*tracingConn)(nil)
	_ driver.Pinger             = (*tracingConn)(nil)
	_ driver.QueryerContext     = (*tracingConn)(nil)
	_ driver.SessionResetter    = (*tracingConn)(nil)
	_ driver.Validator          = (*tracingConn)(nil)
	_ driver.NamedValueChecker  = (*tracingStmt)(nil)
	_ driver.StmtExecContext    = (*tracingStmt)(nil)
	_ driver.StmtQueryContext   = (*tracingStmt)(nil)
)
