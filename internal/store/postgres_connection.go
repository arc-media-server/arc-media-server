package store

import (
	"context"
	"database/sql"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5"
)

// VerifyPostgresConnection checks credentials and proves the candidate reaches the same live
// database, even if a different host serves a restored copy with the same Arc identity.
func (s *Store) VerifyPostgresConnection(ctx context.Context, dsn, id string) error {
	if !s.postgres || id == "" {
		return errPostgresIdentity
	}
	cfg, err := postgresConfig(dsn)
	if err != nil {
		return err
	}
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return errors.New("could not connect to PostgreSQL with these settings")
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = conn.Close(cleanup)
	}()
	if err := requirePostgresIdentity(ctx, conn.QueryRow, id); err != nil {
		return errors.New("this connection must reach Arc's current PostgreSQL database")
	}

	// Advisory locks are local to the database and need no application writes.
	active, err := s.db.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return errors.New("could not verify the connection against Arc's current database")
	}
	defer active.Rollback()
	key := rand.Int64()
	if _, err := active.ExecContext(ctx, "SELECT pg_advisory_xact_lock(?)", key); err != nil {
		// The PostgreSQL store translates placeholders on its connections.
		return errors.New("could not verify the connection against Arc's current database")
	}

	var different bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock($1)", key).Scan(&different); err != nil || different {
		return errors.New("this connection must reach Arc's current PostgreSQL database")
	}
	var canWrite bool
	err = conn.QueryRow(ctx, `
		SELECT current_setting('transaction_read_only') = 'off'
			AND has_schema_privilege('public', 'CREATE')
			AND NOT EXISTS (
				SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
				WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p', 'S')
					AND NOT pg_has_role(c.relowner, 'USAGE')
			)`).Scan(&canWrite)
	if err != nil || !canWrite {
		return errors.New("the PostgreSQL user must have write and schema ownership permissions for Arc's database")
	}
	return nil
}
