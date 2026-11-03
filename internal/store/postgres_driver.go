package store

import (
	"context"
	"crypto/x509"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
)

// Adapt at the driver boundary so every execution path, including transactions and
// prepared statements, shares the same binding and SQLite-compatible expressions.
func init() { sql.Register("arc-postgres", pgDriver{}) }

type pgDriver struct{}

func (pgDriver) Open(dsn string) (driver.Conn, error) {
	c, err := stdlib.GetDefaultDriver().Open(dsn)
	if err != nil {
		return nil, safePGError(err)
	}
	return &pgConn{Conn: c}, nil
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
	if err == nil || errors.Is(err, io.EOF) || errors.Is(err, driver.ErrBadConn) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
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

// sqlTokens preserves quoted strings/identifiers and comments; replacements never touch data.
func sqlTokens(q string) []string {
	var out []string
	for i := 0; i < len(q); {
		start := i
		c := q[i]
		i++
		switch {
		case c == '\'' || c == '"' || c == '`':
			for i < len(q) {
				if q[i] == c {
					i++
					if i < len(q) && q[i] == c {
						i++
						continue
					}
					break
				}
				i++
			}
		case c == '-' && i < len(q) && q[i] == '-':
			for i < len(q) && q[i] != '\n' {
				i++
			}
		case c == '/' && i < len(q) && q[i] == '*':
			i++
			for i+1 < len(q) && q[i:i+2] != "*/" {
				i++
			}
			if i+1 < len(q) {
				i += 2
			}
		case i < len(q) && ((c == '<' && (q[i] == '>' || q[i] == '=')) || (c == '>' && q[i] == '=') || (c == '|' && q[i] == '|') || (c == ':' && q[i] == ':')):
			i++
		case c == '?':
			for i < len(q) && q[i] >= '0' && q[i] <= '9' {
				i++
			}
		case c == ' ' || c == '\n' || c == '\t' || c == '\r':
			for i < len(q) && strings.ContainsRune(" \n\t\r", rune(q[i])) {
				i++
			}
		case c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c >= '0' && c <= '9':
			for i < len(q) {
				v := q[i]
				if !(v >= 'a' && v <= 'z' || v >= 'A' && v <= 'Z' || v == '_' || v >= '0' && v <= '9') {
					break
				}
				i++
			}
		}
		out = append(out, q[start:i])
	}
	return out
}
