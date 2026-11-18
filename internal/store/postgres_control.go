package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"path/filepath"
	"time"
)

type PostgresPreflight struct {
	Version   int  `json:"version"`
	Empty     bool `json:"empty"`
	CanCreate bool `json:"can_create"`
}

func postgresControl(ctx context.Context, dsn string) (*sql.DB, func(), error) {
	c, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, nil, errors.New("invalid PostgreSQL connection configuration")
	}
	c.ConnectTimeout = 5 * time.Second
	c.RuntimeParams["statement_timeout"] = "30000"
	c.DefaultQueryExecMode = pgx.QueryExecModeExec
	id := registerPGConfig(c)
	db, err := sql.Open("arc-postgres", id)
	if err != nil {
		unregisterPGConfig(id)
		return nil, nil, safePGError(err)
	}
	db.SetMaxOpenConns(1)
	cleanup := func() { db.Close(); unregisterPGConfig(id) }
	if err = db.PingContext(ctx); err != nil {
		cleanup()
		return nil, nil, safePGError(err)
	}
	return db, cleanup, nil
}

// InspectPostgres tests connectivity/version/permissions without creating or changing a schema.
func InspectPostgres(ctx context.Context, dsn string) (out PostgresPreflight, err error) {
	db, done, err := postgresControl(ctx, dsn)
	if err != nil {
		return out, err
	}
	defer done()
	if err = db.QueryRowContext(ctx, "SELECT current_setting('server_version_num')::integer").Scan(&out.Version); err != nil {
		return out, err
	}
	if out.Version < 160000 || out.Version >= 190000 {
		return out, errors.New("PostgreSQL 16–18 is required")
	}
	var count int
	if err = db.QueryRowContext(ctx, "SELECT (SELECT count(*) FROM pg_class WHERE relnamespace=current_schema()::regnamespace)+(SELECT count(*) FROM pg_proc WHERE pronamespace=current_schema()::regnamespace)").Scan(&count); err != nil {
		return out, err
	}
	out.Empty = count == 0
	err = db.QueryRowContext(ctx, "SELECT has_schema_privilege(current_schema(),'CREATE')").Scan(&out.CanCreate)
	return out, err
}

// CompletedTransfer verifies a crash-recovered target against its retained SQLite snapshot.
// An ownership ID and complete marker alone are insufficient: recheck every table's contents.
func CompletedTransfer(ctx context.Context, source, dsn, id string) (report TransferReport, complete bool, err error) {
	db, done, err := postgresControl(ctx, dsn)
	if err != nil {
		return report, false, err
	}
	defer done()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return report, false, err
	}
	defer tx.Rollback()
	var locked bool
	if err = tx.QueryRowContext(ctx, "SELECT pg_try_advisory_xact_lock(hashtextextended(current_database() || ':' || current_schema(),308))").Scan(&locked); err != nil || !locked {
		return report, false, errors.New("PostgreSQL target is already in use")
	}
	var exists bool
	if err = tx.QueryRowContext(ctx, "SELECT to_regclass('arc_postgres_schema') IS NOT NULL").Scan(&exists); err != nil {
		return report, false, err
	}
	if !exists {
		return report, false, nil
	}
	var savedID, raw, fingerprint string
	if err = tx.QueryRowContext(ctx, "SELECT transfer_id,transfer_report,fingerprint FROM arc_postgres_schema WHERE singleton=1").Scan(&savedID, &raw, &fingerprint); err != nil {
		return report, false, err
	}
	releases, err := postgresReleases()
	if err != nil {
		return report, false, err
	}
	var release *postgresRelease
	for i := range releases {
		if releases[i].fingerprint == fingerprint {
			release = &releases[i]
			break
		}
	}
	if release == nil {
		return report, false, errors.New("PostgreSQL transfer schema is newer or unsupported")
	}
	if err = verifyPostgresRelease(ctx, tx, *release); err != nil {
		return report, false, err
	}
	layout := releasePortableLayout(*release)
	if savedID != id {
		return report, false, errors.New("PostgreSQL target belongs to another installation or transfer")
	}
	if err = json.Unmarshal([]byte(raw), &report); err != nil || !report.Verified || len(report.Tables) != len(layout.columns) {
		return report, false, errors.New("PostgreSQL transfer marker is incomplete")
	}
	src, err := sql.Open("sqlite", "file:"+filepath.ToSlash(source)+"?mode=ro&_pragma=query_only(1)")
	if err != nil {
		return report, false, err
	}
	defer src.Close()
	read, err := src.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return report, false, err
	}
	defer read.Rollback()
	if err = checkPortableSchemaLayout(ctx, read, layout); err != nil {
		return report, false, err
	}
	for _, name := range layout.tableNames() {
		a, e := tableDigestLayout(ctx, read, name, false, layout)
		if e != nil {
			return report, false, e
		}
		b, e := tableDigestLayout(ctx, tx, name, true, layout)
		if e != nil {
			return report, false, e
		}
		if a != b {
			return report, false, errors.New("PostgreSQL target no longer matches its transfer snapshot")
		}
	}
	return report, true, nil
}
