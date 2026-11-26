package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"ams/internal/dbsql"

	"github.com/jackc/pgx/v5"
)

type postgresStep struct {
	version string
	// Each entry is one transaction-safe PostgreSQL statement.
	statements []string
}

// The initial SQL and applied step definitions are immutable after release. A new Arc
// migration adds one entry here, even if it needs no PostgreSQL statements.
var postgresSteps = []postgresStep{
	{version: "0072_season_label_modes", statements: []string{
		`ALTER TABLE libraries ADD COLUMN season_label_mode TEXT NOT NULL DEFAULT 'number' CHECK (season_label_mode IN ('number', 'name'))`,
		`ALTER TABLE items ADD COLUMN season_label_mode TEXT NOT NULL DEFAULT '' CHECK (season_label_mode IN ('', 'number', 'name'))`,
	}},
	{version: "0073_movie_replacements", statements: []string{
		`ALTER TABLE media_files ADD COLUMN first_seen_at BIGINT`,
		`ALTER TABLE items ADD COLUMN movie_replaced_at BIGINT`,
		`ALTER TABLE items ADD COLUMN movie_replacement_sequence BIGINT`,
	}},
}

type postgresSchema struct {
	versions     []string
	steps        []postgresStep
	fingerprints []string
}

func currentPostgresSchema() (postgresSchema, error) {
	names, err := migrationNames()
	if err != nil {
		return postgresSchema{}, err
	}
	versions := make([]string, len(names))
	for i, name := range names {
		versions[i] = migrationVersion(name)
	}
	return newPostgresSchema(versions, postgresSteps)
}

func newPostgresSchema(versions []string, steps []postgresStep) (postgresSchema, error) {
	initialCount := len(versions) - len(steps)
	if initialCount < 1 || !strings.HasPrefix(postgresBaseline, "-- Arc migrations: "+strings.Join(versions[:initialCount], ",")+"\n") {
		return postgresSchema{}, errors.New("PostgreSQL migration registry does not match Arc's migration history")
	}
	schema := postgresSchema{versions: versions, steps: steps}
	fingerprint := fmt.Sprintf("%x", sha256.Sum256([]byte(postgresBaseline)))
	schema.fingerprints = append(schema.fingerprints, fingerprint)
	for i, step := range steps {
		if step.version != versions[initialCount+i] {
			return postgresSchema{}, fmt.Errorf("PostgreSQL migration missing for %s", versions[initialCount+i])
		}
		h := sha256.New()
		fmt.Fprint(h, fingerprint, "\x00", step.version)
		for _, sql := range step.statements {
			tokens := dbsql.Significant(sql)
			if len(tokens) == 0 {
				return postgresSchema{}, errors.New("empty PostgreSQL migration statement")
			}
			switch strings.ToUpper(tokens[0]) {
			case "ALTER", "CREATE", "DROP", "INSERT", "UPDATE", "DELETE":
			default:
				return postgresSchema{}, errors.New("unsupported PostgreSQL migration statement")
			}
			fmt.Fprint(h, "\x00", sql)
		}
		fingerprint = fmt.Sprintf("%x", h.Sum(nil))
		schema.fingerprints = append(schema.fingerprints, fingerprint)
	}
	return schema, nil
}

func checkPostgresVersion(ctx context.Context, tx pgx.Tx) error {
	var version int
	if err := tx.QueryRow(ctx, "SELECT current_setting('server_version_num')::integer").Scan(&version); err != nil {
		return safePGError(err)
	}
	if version < 160000 || version >= 200000 {
		return errors.New("Arc requires PostgreSQL 16 through 19")
	}
	return nil
}

func requireEmptyPostgres(ctx context.Context, tx pgx.Tx) error {
	var objects int
	const existingObjects = `SELECT (SELECT count(*) FROM pg_class WHERE relnamespace='public'::regnamespace)
		+ (SELECT count(*) FROM pg_proc WHERE pronamespace='public'::regnamespace)
		+ (SELECT count(*) FROM pg_type WHERE typnamespace='public'::regnamespace)`
	if err := tx.QueryRow(ctx, existingObjects).Scan(&objects); err != nil {
		return safePGError(err)
	}
	if objects != 0 {
		return errors.New("PostgreSQL requires a dedicated empty database")
	}
	return nil
}

func applyPostgresSteps(ctx context.Context, tx pgx.Tx, steps []postgresStep) error {
	for _, step := range steps {
		for _, sql := range step.statements {
			// The extended protocol accepts a single statement. Its parse step rejects
			// a second statement that could otherwise commit an upgrade partway through.
			results := tx.Conn().PgConn().ExecParams(ctx, sql, nil, nil, nil, nil).Read()
			if results.Err != nil {
				return fmt.Errorf("PostgreSQL migration %s: %w", step.version, safePGError(results.Err))
			}
		}
	}
	return nil
}

func createPostgresTables(ctx context.Context, tx pgx.Tx, steps []postgresStep) error {
	if _, err := tx.Exec(ctx, postgresBaseline); err != nil {
		return safePGError(err)
	}
	if _, err := tx.Exec(ctx, "CREATE TABLE arc_postgres_schema (singleton integer PRIMARY KEY CHECK (singleton=1), fingerprint text NOT NULL, layout_hash text NOT NULL)"); err != nil {
		return safePGError(err)
	}
	return applyPostgresSteps(ctx, tx, steps)
}

func postgresLevel(ctx context.Context, tx pgx.Tx, schema postgresSchema) (int, error) {
	var fingerprint, savedLayout string
	if err := tx.QueryRow(ctx, "SELECT fingerprint, layout_hash FROM arc_postgres_schema WHERE singleton=1").Scan(&fingerprint, &savedLayout); err != nil {
		return 0, safePGError(err)
	}
	for level, known := range schema.fingerprints {
		if fingerprint == known {
			count := len(schema.versions) - len(schema.steps) + level
			rows, err := tx.Query(ctx, "SELECT version FROM schema_migrations ORDER BY version")
			if err != nil {
				return 0, safePGError(err)
			}
			versions, err := pgx.CollectRows(rows, pgx.RowTo[string])
			if err != nil {
				return 0, safePGError(err)
			}
			if len(versions) != count || strings.Join(versions, ",") != strings.Join(schema.versions[:count], ",") {
				return 0, errors.New("PostgreSQL migration history does not match its Arc schema")
			}
			if layout, err := postgresLayoutHash(ctx, tx); err != nil {
				return 0, err
			} else if layout != savedLayout {
				return 0, errors.New("PostgreSQL table or sequence layout differs from its Arc schema")
			}
			return level, nil
		}
	}
	return 0, errors.New("PostgreSQL schema is newer or unsupported by this Arc build")
}

func recordPostgresVersions(ctx context.Context, tx pgx.Tx, versions []string) error {
	now := time.Now().Unix()
	for _, version := range versions {
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations(version,applied_at) VALUES($1,$2)", version, now); err != nil {
			return safePGError(err)
		}
	}
	return nil
}

func initializePostgres(ctx context.Context, tx pgx.Tx, backupDir string, schema postgresSchema, binding postgresBinding) error {
	if err := checkPostgresVersion(ctx, tx); err != nil {
		return err
	}
	var exists bool
	if err := tx.QueryRow(ctx, "SELECT to_regclass('public.arc_postgres_schema') IS NOT NULL").Scan(&exists); err != nil {
		return safePGError(err)
	}
	current := len(schema.steps)
	if exists {
		if binding.publish != nil {
			return errors.New("PostgreSQL requires a dedicated empty database")
		}
		if binding.id != "" {
			if err := requirePostgresIdentity(ctx, tx.QueryRow, binding.id); err != nil {
				return err
			}
		}
		level, err := postgresLevel(ctx, tx, schema)
		if err != nil || level == current {
			return err
		}
		if backupDir == "" {
			return errors.New("PostgreSQL upgrade requires a safety backup directory")
		}
		if err := backupPostgresTx(ctx, tx, backupDir, schema.fingerprints[level]); err != nil {
			return fmt.Errorf("PostgreSQL before-upgrade backup: %w", err)
		}
		if err := applyPostgresSteps(ctx, tx, schema.steps[level:]); err != nil {
			return err
		}
		if err := recordPostgresVersions(ctx, tx, schema.versions[len(schema.versions)-current+level:]); err != nil {
			return err
		}
		layout, err := postgresLayoutHash(ctx, tx)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "UPDATE arc_postgres_schema SET fingerprint=$1, layout_hash=$2 WHERE singleton=1", schema.fingerprints[current], layout)
		return safePGError(err)
	}

	if binding.id != "" && !binding.pending {
		return errors.New("the selected PostgreSQL database is no longer initialized")
	}
	if err := requireEmptyPostgres(ctx, tx); err != nil {
		return err
	}
	if err := createPostgresTables(ctx, tx, schema.steps); err != nil {
		return err
	}
	if err := recordPostgresVersions(ctx, tx, schema.versions); err != nil {
		return err
	}
	id := binding.id
	if id == "" {
		id = NewID()
	}
	for key, value := range map[string]string{"server_id": id, "background_epoch": fmt.Sprint(time.Now().Unix())} {
		if _, err := tx.Exec(ctx, "INSERT INTO settings(key,value) VALUES($1,$2)", key, value); err != nil {
			return safePGError(err)
		}
	}
	if _, err := tx.Exec(ctx, "INSERT INTO arrival_clock(id,sequence) VALUES(1,0)"); err != nil {
		return safePGError(err)
	}
	layout, err := postgresLayoutHash(ctx, tx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "INSERT INTO arc_postgres_schema(singleton,fingerprint,layout_hash) VALUES(1,$1,$2)", schema.fingerprints[current], layout); err != nil {
		return safePGError(err)
	}
	if binding.publish != nil {
		return binding.publish()
	}
	return nil
}
