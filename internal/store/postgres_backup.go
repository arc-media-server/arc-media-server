package store

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type postgresColumn struct {
	Name     string
	Type     string
	Required bool
}

type postgresTable struct {
	Name    string
	Columns []postgresColumn
	Rows    int64
}

func (t postgresTable) copyTarget() string {
	names := make([]string, len(t.Columns))
	for i, col := range t.Columns {
		names[i] = pgx.Identifier{col.Name}.Sanitize()
	}
	return pgx.Identifier{"public", t.Name}.Sanitize() + " (" + strings.Join(names, ",") + ")"
}

type postgresSequence struct {
	Name   string
	Value  int64
	Called bool
}

type postgresBackupManifest struct {
	Format      int
	Fingerprint string
	Tables      []postgresTable
	Sequences   []postgresSequence
}

func postgresTables(ctx context.Context, tx pgx.Tx) ([]postgresTable, error) {
	const query = `SELECT c.relname,a.attname,format_type(a.atttypid,a.atttypmod),a.attnotnull,a.attgenerated::text
		FROM pg_class c JOIN pg_attribute a ON a.attrelid=c.oid
		WHERE c.relnamespace='public'::regnamespace AND c.relkind='r'
		  AND a.attnum>0 AND NOT a.attisdropped
		ORDER BY c.relname COLLATE "C",a.attnum`
	rows, err := tx.Query(ctx, query)
	if err != nil {
		return nil, safePGError(err)
	}
	defer rows.Close()
	var tables []postgresTable
	for rows.Next() {
		var name, generated string
		var column postgresColumn
		if err := rows.Scan(&name, &column.Name, &column.Type, &column.Required, &generated); err != nil {
			return nil, safePGError(err)
		}
		if generated != "" {
			return nil, errors.New("PostgreSQL backup found an unsupported generated column")
		}
		if len(tables) == 0 || tables[len(tables)-1].Name != name {
			tables = append(tables, postgresTable{Name: name})
		}
		table := &tables[len(tables)-1]
		table.Columns = append(table.Columns, column)
	}
	return tables, safePGError(rows.Err())
}

func postgresSequences(ctx context.Context, tx pgx.Tx) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT relname FROM pg_class WHERE relkind='S'
		AND relnamespace='public'::regnamespace ORDER BY relname COLLATE "C"`)
	if err != nil {
		return nil, safePGError(err)
	}
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	return names, safePGError(err)
}

func postgresLayoutHash(ctx context.Context, tx pgx.Tx) (string, error) {
	tables, err := postgresTables(ctx, tx)
	if err != nil {
		return "", err
	}
	names, err := postgresSequences(ctx, tx)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	for _, value := range []any{tables, names} {
		if err := json.NewEncoder(h).Encode(value); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// IsPostgres reports the active store backend without exposing connection details.
func (s *Store) IsPostgres() bool { return s.postgres }

func (s *Store) backupPostgres(ctx context.Context, path string) error {
	schema, err := currentPostgresSchema()
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	success := false
	defer func() {
		if !success {
			file.Close()
			os.Remove(path)
		}
	}()
	// A read-only snapshot lets watch progress and other writes continue.
	err = postgresTransaction(ctx, s.db.DB, func(tx pgx.Tx) error {
		level, err := postgresLevel(ctx, tx, schema)
		if err != nil {
			return err
		}
		fingerprint := schema.fingerprints[level]
		if err := writePostgresArchive(ctx, tx, file, fingerprint); err != nil {
			return err
		}
		if err := file.Sync(); err != nil {
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		return verifyPostgresArchive(ctx, path, fingerprint)
	})
	success = err == nil
	return err
}

func backupPostgresTx(ctx context.Context, tx pgx.Tx, dir, fingerprint string) error {
	if err := os.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	temp, err := os.CreateTemp(dir, ".pg-before-upgrade-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	if err := writePostgresArchive(ctx, tx, temp, fingerprint); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := verifyPostgresArchive(ctx, temp.Name(), fingerprint); err != nil {
		return err
	}

	suffix := strings.TrimPrefix(filepath.Base(temp.Name()), ".pg-before-upgrade-")
	name := "ams-" + time.Now().Format("20060102-150405") + "-" + suffix + "-before-upgrade.pg.zip"
	if err := os.Rename(temp.Name(), filepath.Join(dir, name)); err != nil {
		return err
	}
	// Sync both the archive entry and the backup directory entry before changing the database.
	if err := syncPostgresBackupDir(dir); err != nil {
		return err
	}
	return syncPostgresBackupDir(filepath.Dir(dir))
}

func syncPostgresBackupDir(path string) error {
	folder, err := os.Open(path)
	if err != nil {
		return err
	}
	defer folder.Close()
	return folder.Sync()
}

func writePostgresArchive(ctx context.Context, tx pgx.Tx, destination io.Writer, fingerprint string) error {
	tables, err := postgresTables(ctx, tx)
	if err != nil {
		return err
	}
	sequenceNames, err := postgresSequences(ctx, tx)
	if err != nil {
		return err
	}
	manifest := postgresBackupManifest{Format: 1, Fingerprint: fingerprint, Tables: tables}
	for _, name := range sequenceNames {
		sequence := postgresSequence{Name: name}
		q := "SELECT last_value, is_called FROM " + pgx.Identifier{"public", name}.Sanitize()
		if err := tx.QueryRow(ctx, q).Scan(&sequence.Value, &sequence.Called); err != nil {
			return safePGError(err)
		}
		manifest.Sequences = append(manifest.Sequences, sequence)
	}

	z := zip.NewWriter(destination)
	for i, table := range tables {
		entry, err := z.Create(postgresTableEntry(i))
		if err != nil {
			return err
		}
		result, err := tx.Conn().PgConn().CopyTo(ctx, entry, "COPY "+table.copyTarget()+" TO STDOUT WITH (FORMAT CSV)")
		if err != nil {
			return safePGError(err)
		}
		manifest.Tables[i].Rows = result.RowsAffected()
	}
	entry, err := z.Create("manifest.json")
	if err != nil {
		return err
	}
	if err := json.NewEncoder(entry).Encode(manifest); err != nil {
		return err
	}
	return z.Close()
}

func postgresTableEntry(i int) string { return fmt.Sprintf("table-%d.csv", i) }

func postgresManifest(z *zip.Reader) (postgresBackupManifest, error) {
	var manifest postgresBackupManifest
	entry, err := z.Open("manifest.json")
	if err != nil {
		return manifest, err
	}
	defer entry.Close()
	const maxManifest = 1 << 20
	b, err := io.ReadAll(io.LimitReader(entry, maxManifest+1))
	if err != nil {
		return manifest, err
	}
	if len(b) > maxManifest || json.Unmarshal(b, &manifest) != nil || manifest.Format != 1 ||
		len(manifest.Tables) == 0 || len(z.File) != len(manifest.Tables)+1 {
		return manifest, errors.New("invalid Arc PostgreSQL backup manifest")
	}
	return manifest, nil
}

func verifyPostgresArchive(ctx context.Context, path, fingerprint string) error {
	z, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer z.Close()
	manifest, err := postgresManifest(&z.Reader)
	if err != nil {
		return err
	}
	if manifest.Fingerprint != fingerprint {
		return errors.New("PostgreSQL backup schema does not match")
	}
	for i := range manifest.Tables {
		if err := ctx.Err(); err != nil {
			return err
		}
		entry, err := z.Open(postgresTableEntry(i))
		if err != nil {
			return err
		}
		_, readErr := io.Copy(io.Discard, entry) // Read through EOF to check ZIP integrity.
		entry.Close()
		if readErr != nil {
			return readErr
		}
	}
	return nil
}

// RestorePostgresBackup restores an Arc safety archive into a dedicated empty PostgreSQL database.
// This never writes to the source installation and does not switch Arc's chosen database.
func RestorePostgresBackup(ctx context.Context, dsn, path string) error {
	schema, err := currentPostgresSchema()
	if err != nil {
		return err
	}
	return restorePostgresBackup(ctx, dsn, path, schema)
}

func restorePostgresBackup(ctx context.Context, dsn, path string, schema postgresSchema) error {
	z, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer z.Close()
	manifest, err := postgresManifest(&z.Reader)
	if err != nil {
		return err
	}
	level := slices.Index(schema.fingerprints, manifest.Fingerprint)
	if level == -1 {
		return errors.New("PostgreSQL backup is from an unsupported Arc schema")
	}
	cfg, err := postgresConfig(dsn)
	if err != nil {
		return err
	}
	writer := postgresWriter(cfg)
	defer writer.Close()
	return postgresTransaction(ctx, writer, func(tx pgx.Tx) error {
		if err := checkPostgresVersion(ctx, tx); err != nil {
			return err
		}
		if err := requireEmptyPostgres(ctx, tx); err != nil {
			return err
		}
		if err := createPostgresTables(ctx, tx, schema.steps[:level]); err != nil {
			return err
		}
		if err := restorePostgresTables(ctx, tx, &z.Reader, manifest); err != nil {
			return err
		}
		actual, err := postgresLevel(ctx, tx, schema)
		if err == nil && actual != level {
			return errors.New("restored PostgreSQL schema does not match the backup")
		}
		return err
	})
}

func restorePostgresTables(ctx context.Context, tx pgx.Tx, z *zip.Reader, manifest postgresBackupManifest) error {
	tables, err := postgresTables(ctx, tx)
	if err != nil {
		return err
	}
	if len(tables) != len(manifest.Tables) {
		return errors.New("PostgreSQL backup table inventory does not match")
	}
	for i, table := range tables {
		saved := manifest.Tables[i]
		if table.Name != saved.Name || !slices.Equal(table.Columns, saved.Columns) {
			return errors.New("PostgreSQL backup columns do not match")
		}
		entry, err := z.Open(postgresTableEntry(i))
		if err != nil {
			return err
		}
		result, copyErr := tx.Conn().PgConn().CopyFrom(ctx, entry, "COPY "+table.copyTarget()+" FROM STDIN WITH (FORMAT CSV)")
		entry.Close()
		if copyErr != nil {
			return safePGError(copyErr)
		}
		if result.RowsAffected() != saved.Rows {
			return errors.New("PostgreSQL backup row count does not match")
		}
	}
	sequences, err := postgresSequences(ctx, tx)
	if err != nil {
		return err
	}
	if len(sequences) != len(manifest.Sequences) {
		return errors.New("PostgreSQL backup sequences do not match")
	}
	for i, name := range sequences {
		saved := manifest.Sequences[i]
		if name != saved.Name {
			return errors.New("PostgreSQL backup sequence name does not match")
		}
		if _, err := tx.Exec(ctx, "SELECT setval($1::regclass,$2,$3)", pgx.Identifier{"public", name}.Sanitize(), saved.Value, saved.Called); err != nil {
			return safePGError(err)
		}
	}
	return nil
}
