package store

import (
	"context"
	"database/sql"
	"errors"
)

// database keeps reads independent of the single SQLite writer. database/sql
// queues write requests with context cancellation, rather than opening competing
// writers that can exhaust busy_timeout during scans and maintenance.
// Query/QueryRow are read-only; statements returning written rows use WriteRowContext.
type database struct {
	*sql.DB
	writer *sql.DB
}

func openDatabase(ctx context.Context, dsn string) (*database, error) {
	writer, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	writer.SetMaxOpenConns(1)
	writer.SetMaxIdleConns(1)
	// Establish WAL before opening read connections with query_only enabled.
	if err := writer.PingContext(ctx); err != nil {
		writer.Close()
		return nil, err
	}
	reader, err := sql.Open("sqlite", dsn+"&_pragma=query_only(1)")
	if err != nil {
		writer.Close()
		return nil, err
	}
	return &database{DB: reader, writer: writer}, nil
}

func (d *database) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return d.writer.ExecContext(ctx, query, args...)
}

func (d *database) Exec(query string, args ...any) (sql.Result, error) {
	return d.ExecContext(context.Background(), query, args...)
}

func (d *database) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return d.writer.BeginTx(ctx, opts)
}

func (d *database) Begin() (*sql.Tx, error) {
	return d.BeginTx(context.Background(), nil)
}

func (d *database) WriteRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return d.writer.QueryRowContext(ctx, query, args...)
}

// Raw connection access is for online restore and must join the writer queue too.
func (d *database) Conn(ctx context.Context) (*sql.Conn, error) {
	return d.writer.Conn(ctx)
}

func (d *database) Close() error {
	return errors.Join(d.DB.Close(), d.writer.Close())
}
