package store

import (
	"context"
	"crypto/x509"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// All store executions, including inside a transaction, use the same SQL adapter.
type postgresConnector struct{ driver.Connector }

func (c postgresConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, safePGError(err)
	}
	return &pgConn{Conn: conn}, nil
}

type pgConn struct{ driver.Conn }

func (c *pgConn) ExecContext(ctx context.Context, q string, a []driver.NamedValue) (driver.Result, error) {
	q, a = postgresSQL(q, a)
	r, e := c.Conn.(driver.ExecerContext).ExecContext(ctx, q, a)
	return r, safePGError(e)
}
func (c *pgConn) QueryContext(ctx context.Context, q string, a []driver.NamedValue) (driver.Rows, error) {
	q, a = postgresSQL(q, a)
	r, e := c.Conn.(driver.QueryerContext).QueryContext(ctx, q, a)
	if e != nil {
		return nil, safePGError(e)
	}
	return &pgRows{Rows: r}, nil
}
func (c *pgConn) BeginTx(ctx context.Context, o driver.TxOptions) (driver.Tx, error) {
	tx, e := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, o)
	if e != nil {
		return nil, safePGError(e)
	}
	return pgTx{tx}, nil
}
func (c *pgConn) Ping(ctx context.Context) error {
	return safePGError(c.Conn.(driver.Pinger).Ping(ctx))
}
func (c *pgConn) ResetSession(ctx context.Context) error {
	return safePGError(c.Conn.(driver.SessionResetter).ResetSession(ctx))
}
func (c *pgConn) IsValid() bool {
	if v, ok := c.Conn.(driver.Validator); ok {
		return v.IsValid()
	}
	return true
}

// All store calls use Context execution. Explicit preparation must not bypass dialect binding.
func (c *pgConn) Prepare(q string) (driver.Stmt, error) { return &pgStmt{c: c, q: q}, nil }
func (c *pgConn) PrepareContext(_ context.Context, q string) (driver.Stmt, error) {
	return c.Prepare(q)
}

type pgStmt struct {
	c *pgConn
	q string
}

func (s *pgStmt) Close() error  { return nil }
func (s *pgStmt) NumInput() int { return -1 }
func (s *pgStmt) Exec(a []driver.Value) (driver.Result, error) {
	return s.ExecContext(context.Background(), namedValues(a))
}
func (s *pgStmt) Query(a []driver.Value) (driver.Rows, error) {
	return s.QueryContext(context.Background(), namedValues(a))
}
func (s *pgStmt) ExecContext(ctx context.Context, a []driver.NamedValue) (driver.Result, error) {
	return s.c.ExecContext(ctx, s.q, a)
}
func (s *pgStmt) QueryContext(ctx context.Context, a []driver.NamedValue) (driver.Rows, error) {
	return s.c.QueryContext(ctx, s.q, a)
}
func namedValues(a []driver.Value) []driver.NamedValue {
	out := make([]driver.NamedValue, len(a))
	for i, v := range a {
		out[i] = driver.NamedValue{Ordinal: i + 1, Value: v}
	}
	return out
}

type pgTx struct{ driver.Tx }

func (t pgTx) Commit() error   { return safePGError(t.Tx.Commit()) }
func (t pgTx) Rollback() error { return safePGError(t.Tx.Rollback()) }

type pgRows struct{ driver.Rows }

func (r *pgRows) Close() error { return safePGError(r.Rows.Close()) }

func (r *pgRows) Next(v []driver.Value) error {
	err := r.Rows.Next(v)
	for i, x := range v {
		if b, ok := x.(bool); ok {
			if b {
				v[i] = int64(1)
			} else {
				v[i] = int64(0)
			}
		}
	}
	return safePGError(err)
}
func safePGError(err error) error {
	if err == nil {
		return nil
	}
	for _, sentinel := range []error{errPostgresInUse, errPostgresIdentity, io.EOF, driver.ErrBadConn, context.Canceled, context.DeadlineExceeded} {
		if errors.Is(err, sentinel) {
			return sentinel
		}
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "28P01", "28000":
			return fmt.Errorf("PostgreSQL authentication failed; check the account and password (SQLSTATE %s)", pg.Code)
		case "3D000":
			return errors.New("PostgreSQL database was not found (SQLSTATE 3D000)")
		}
		return fmt.Errorf("PostgreSQL operation failed (SQLSTATE %s)", pg.Code)
	}
	var unknown x509.UnknownAuthorityError
	var host x509.HostnameError
	var invalid x509.CertificateInvalidError
	if errors.As(err, &unknown) {
		return errors.New("PostgreSQL TLS certificate is not trusted; configure its CA in the Arc server trust store")
	}
	if errors.As(err, &host) {
		return errors.New("PostgreSQL TLS certificate does not match the configured host")
	}
	if errors.As(err, &invalid) {
		return errors.New("PostgreSQL TLS certificate validation failed")
	}
	if strings.Contains(err.Error(), "server refused TLS") {
		return errors.New("PostgreSQL server refused required TLS; enable TLS or explicitly disable it only on a trusted network")
	}
	if lower := strings.ToLower(err.Error()); strings.Contains(lower, "tls") || strings.Contains(lower, "x509:") || strings.Contains(lower, "certificate") {
		return errors.New("PostgreSQL TLS connection failed; check the server certificate, hostname and trust configuration")
	}
	var network *net.OpError
	if errors.As(err, &network) {
		return errors.New("PostgreSQL network connection failed; check the host, port and server availability")
	}
	// Driver/network errors can embed connection strings, passwords or row values.
	return errors.New("PostgreSQL connection or operation failed; check database availability, TLS and credentials")
}
