package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

//go:embed postgres/baseline.sql
var postgresBaseline string

var errPostgresMaintenance = errors.New("this SQLite maintenance operation is unavailable for PostgreSQL")
var errPostgresInUse = errors.New("PostgreSQL database is already in use by Arc")

// OpenPostgres initializes a dedicated database or opens a recognized Arc schema.
// A backup directory is required when an existing database needs a schema upgrade.
func OpenPostgres(ctx context.Context, dsn, backupDir string) (*Store, error) {
	schema, err := currentPostgresSchema()
	if err != nil {
		return nil, err
	}
	return openPostgres(ctx, dsn, backupDir, schema, postgresBinding{})
}

// OpenSelectedPostgres binds the external database to the local installation identity.
// On a new selection, publish persists the pending choice before PostgreSQL commits.
func OpenSelectedPostgres(ctx context.Context, dsn, backupDir, id string, pending bool, publish func() error) (*Store, error) {
	schema, err := currentPostgresSchema()
	if err != nil {
		return nil, err
	}
	if id == "" {
		return nil, errors.New("PostgreSQL installation identity is required")
	}
	return openPostgres(ctx, dsn, backupDir, schema, postgresBinding{id: id, pending: pending, publish: publish})
}

type postgresBinding struct {
	id      string
	pending bool
	publish func() error
}

func openPostgres(ctx context.Context, dsn, backupDir string, schema postgresSchema, binding postgresBinding) (*Store, error) {
	cfg, err := postgresConfig(dsn)
	if err != nil {
		return nil, err
	}
	writer := postgresWriter(cfg)
	readConfig := cfg.Copy()
	readConfig.RuntimeParams["default_transaction_read_only"] = "on"
	reader := sql.OpenDB(postgresConnector{stdlib.GetConnector(*readConfig)})
	reader.SetMaxOpenConns(8)
	reader.SetMaxIdleConns(2)

	s := &Store{db: &database{DB: reader, writer: writer}, postgres: true}
	if err := postgresTransaction(ctx, writer, func(tx pgx.Tx) error {
		return initializePostgres(ctx, tx, backupDir, schema, binding)
	}); err != nil {
		s.Close()
		return nil, err
	}
	if err := s.syncMinAges(ctx); err != nil {
		s.Close()
		return nil, err
	}
	s.sessionUses = newSessionUseQueue(s)
	return s, nil
}

func postgresConfig(dsn string) (*pgx.ConnConfig, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, errors.New("invalid PostgreSQL connection configuration")
	}
	cfg.ConnectTimeout = 5 * time.Second
	cfg.RuntimeParams["search_path"] = "public"
	cfg.RuntimeParams["application_name"] = "arc-media-server"
	cfg.RuntimeParams["statement_timeout"] = "30000"
	cfg.RuntimeParams["idle_in_transaction_session_timeout"] = "60000"
	cfg.DefaultQueryExecMode = pgx.QueryExecModeExec

	return cfg, nil
}

func postgresWriter(cfg *pgx.ConnConfig) *sql.DB {
	writer := sql.OpenDB(postgresConnector{stdlib.GetConnector(*cfg, stdlib.OptionAfterConnect(lockPostgresWriter))})
	writer.SetMaxOpenConns(1)
	writer.SetMaxIdleConns(1)
	return writer
}

func postgresTransaction(ctx context.Context, writer *sql.DB, run func(pgx.Tx) error) error {
	conn, err := writer.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.Raw(func(dc any) error {
		pg := dc.(*pgConn).Conn.(*stdlib.Conn).Conn()
		tx, err := pg.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
		if err != nil {
			return safePGError(err)
		}
		defer func() {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			_ = tx.Rollback(cleanup)
		}()
		if err := run(tx); err != nil {
			return err
		}
		return safePGError(tx.Commit(ctx))
	})
}

// The sole write connection owns the lock. A reconnect reacquires it before running any query.
func lockPostgresWriter(ctx context.Context, conn *pgx.Conn) error {
	var locked bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock(hashtextextended(current_database() || ':public', 308))").Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return errPostgresInUse
	}
	return nil
}
