package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

type TableTransfer struct {
	Table  string `json:"table"`
	Rows   int64  `json:"rows"`
	SHA256 string `json:"sha256"`
}
type TransferReport struct {
	Tables   []TableTransfer `json:"tables"`
	Rows     int64           `json:"rows"`
	Verified bool            `json:"verified"`
}
type querySource interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

// TransferSQLite publishes a PostgreSQL schema only after every persistent table's canonical
// contents and constraints have passed validation. The SQLite input is opened read-only.
// Use a consistent Arc safety backup, never a filesystem copy of a live WAL database.
func TransferSQLite(ctx context.Context, source, dsn string) (TransferReport, error) {
	return TransferSQLiteWithID(ctx, source, dsn, NewID())
}
func TransferSQLiteWithID(ctx context.Context, source, dsn, id string) (report TransferReport, err error) {
	if err = CheckBackup(ctx, source); err != nil {
		return report, err
	}
	src, err := sql.Open("sqlite", "file:"+filepath.ToSlash(source)+"?mode=ro&_pragma=query_only(1)")
	if err != nil {
		return report, err
	}
	defer src.Close()
	read, err := src.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return report, err
	}
	defer read.Rollback()
	if err = checkPortableSchema(ctx, read); err != nil {
		return report, err
	}
	st, err := openPostgres(ctx, dsn, Options{}, func(tx *sql.Tx) error {
		if err := clearPortable(ctx, tx, true); err != nil {
			return err
		}
		var e error
		report, e = copyPortable(ctx, read, tx, false, true)
		if e != nil {
			return e
		}
		b, e := json.Marshal(report)
		if e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, "UPDATE arc_postgres_schema SET transfer_id=?,transfer_report=? WHERE singleton=1", id, string(b))
		return e
	})
	if err != nil {
		return TransferReport{}, err
	}
	defer st.Close()
	return report, nil
}

func (s *Store) backupPostgres(ctx context.Context, path string) (err error) {
	// Reserve an owner-only destination and remove incomplete data on every failure.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	f.Close()
	complete := false
	defer func() {
		if !complete {
			os.Remove(path)
			os.Remove(path + "-wal")
			os.Remove(path + "-shm")
		}
	}()
	dest, err := Open(ctx, path, Options{})
	if err != nil {
		return err
	}
	if err = dest.Close(); err != nil {
		return err
	}
	sqlite, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_txlock=immediate")
	if err != nil {
		return err
	}
	defer sqlite.Close()
	read, err := s.db.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return safePGError(err)
	}
	defer read.Rollback()
	write, err := sqlite.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer write.Rollback()
	if _, err = write.ExecContext(ctx, "PRAGMA defer_foreign_keys=ON"); err != nil {
		return err
	}
	if err = clearPortable(ctx, write, false); err != nil {
		return err
	}
	if _, err = copyPortable(ctx, read, write, true, false); err != nil {
		return err
	}
	if err = write.Commit(); err != nil {
		return err
	}
	if _, err = sqlite.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return err
	}
	if err = CheckBackup(ctx, path); err != nil {
		return err
	}
	complete = true
	return nil
}
func (s *Store) restorePostgres(ctx context.Context, path string) error {
	upgraded, cleanup, err := upgradedPortableBackup(ctx, path)
	if err != nil {
		return err
	}
	defer cleanup()
	path = upgraded

	if err := CheckBackup(ctx, path); err != nil {
		return err
	}
	src, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro&_pragma=query_only(1)")
	if err != nil {
		return err
	}
	defer src.Close()
	read, err := src.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer read.Rollback()
	if err = checkPortableSchema(ctx, read); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = clearPortable(ctx, tx, true); err != nil {
		return err
	}
	if _, err = copyPortable(ctx, read, tx, false, true); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return s.syncMinAges(ctx)
}
func checkPortableSchema(ctx context.Context, src querySource) error {
	rows, err := src.QueryContext(ctx, "SELECT name FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return err
	}
	defer rows.Close()
	seen := map[string]bool{}
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return err
		}
		if _, ok := pgColumns[name]; !ok {
			return errors.New("SQLite contains an unsupported table; transfer requires the matching Arc schema")
		}
		seen[name] = true
	}
	if err = rows.Err(); err != nil {
		return err
	}
	fk, e := src.QueryContext(ctx, "PRAGMA foreign_key_check")
	if e != nil {
		return e
	}
	bad := fk.Next()
	e = fk.Err()
	fk.Close()
	if e != nil {
		return e
	}
	if bad {
		return errors.New("SQLite source has broken references; repair or restore it before transferring")
	}
	if len(seen) != len(pgColumns) {
		return errors.New("SQLite schema must match this Arc version before PostgreSQL transfer")
	}
	// Check names/order, not just the number of tables: no persistent column is silently lost.
	for name, want := range pgColumns {
		r, e := src.QueryContext(ctx, "SELECT * FROM "+quoted(name)+" LIMIT 0")
		if e != nil {
			return e
		}
		got, e := r.Columns()
		r.Close()
		if e != nil {
			return e
		}
		if strings.Join(want, "\x00") != strings.Join(got, "\x00") {
			return fmt.Errorf("table %s schema differs; use matching Arc version", name)
		}
	}
	return nil
}
func clearPortable(ctx context.Context, tx *sql.Tx, postgres bool) error {
	// DELETE preserves old MVCC snapshots during live PostgreSQL restore. TRUNCATE does not.
	if postgres {
		if _, err := tx.ExecContext(ctx, "SET CONSTRAINTS ALL DEFERRED"); err != nil {
			return err
		}
	}
	for _, name := range tableNames() {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+quoted(name)); err != nil {
			return err
		}
	}
	return nil
}

func portableCols(table string, postgres bool) (selectCols, insertCols, order string) {
	var cols []string
	for _, c := range pgColumns[table] {
		cols = append(cols, quoted(c))
	}
	orderFields := make([]string, len(cols))
	for i, c := range cols {
		collation := ""
		if portableTextColumn(table, pgColumns[table][i]) {
			if postgres {
				collation = ` COLLATE "C"`
			} else {
				collation = " COLLATE BINARY"
			}
		}
		orderFields[i] = c + collation + " NULLS FIRST"
	}
	selectCols, insertCols = strings.Join(cols, ","), strings.Join(cols, ",")
	if hasPortableRowID(table) {
		col := "rowid"
		if postgres {
			col = "arc_rowid"
		}
		selectCols += "," + col
		insertCols += "," + col
		orderFields = append(orderFields, col+" NULLS FIRST")
	}
	order = strings.Join(orderFields, ",")
	return
}
func canonicalRow(hash io.Writer, values []any) error {
	b, err := json.Marshal(values)
	if err != nil {
		return err
	}
	fmt.Fprintf(hash, "%d:", len(b))
	_, err = hash.Write(b)
	return err
}
func tableDigest(ctx context.Context, src querySource, table string, postgres bool) (TableTransfer, error) {
	out := TableTransfer{Table: table}
	cols, _, order := portableCols(table, postgres)
	rows, err := src.QueryContext(ctx, "SELECT "+cols+" FROM "+quoted(table)+" ORDER BY "+order)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	h := sha256.New()
	names, err := rows.Columns()
	if err != nil {
		return out, err
	}
	values := make([]any, len(names))
	ptr := make([]any, len(names))
	for i := range values {
		ptr[i] = &values[i]
	}
	for rows.Next() {
		if err = rows.Scan(ptr...); err != nil {
			return out, err
		}
		if err = canonicalRow(h, values); err != nil {
			return out, err
		}
		out.Rows++
	}
	out.SHA256 = hex.EncodeToString(h.Sum(nil))
	return out, rows.Err()
}
func copyPortable(ctx context.Context, src querySource, dest *sql.Tx, sourcePG, destPG bool) (TransferReport, error) {
	report := TransferReport{Tables: []TableTransfer{}}
	for _, table := range tableNames() {
		cols, _, order := portableCols(table, sourcePG)
		_, target, _ := portableCols(table, destPG)
		rows, err := src.QueryContext(ctx, "SELECT "+cols+" FROM "+quoted(table)+" ORDER BY "+order)
		if err != nil {
			return report, err
		}
		names, err := rows.Columns()
		if err != nil {
			rows.Close()
			return report, err
		}
		placeholders := make([]string, len(names))
		for i := range placeholders {
			placeholders[i] = "?"
		}
		stmt, err := dest.PrepareContext(ctx, "INSERT INTO "+quoted(table)+" ("+target+") VALUES ("+strings.Join(placeholders, ",")+")")
		if err != nil {
			rows.Close()
			return report, err
		}
		values := make([]any, len(names))
		ptr := make([]any, len(names))
		for i := range values {
			ptr[i] = &values[i]
		}
		h := sha256.New()
		result := TableTransfer{Table: table}
		for rows.Next() {
			if err = ctx.Err(); err != nil {
				break
			}
			if err = rows.Scan(ptr...); err != nil {
				break
			}
			if _, err = stmt.ExecContext(ctx, values...); err != nil {
				break
			}
			if err = canonicalRow(h, values); err != nil {
				break
			}
			result.Rows++
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		stmt.Close()
		if err != nil {
			return report, fmt.Errorf("copying table %s: %w", table, err)
		}
		result.SHA256 = hex.EncodeToString(h.Sum(nil))
		report.Tables = append(report.Tables, result)
		report.Rows += result.Rows
	}
	for _, want := range report.Tables {
		got, err := tableDigest(ctx, dest, want.Table, destPG)
		if err != nil {
			return report, err
		}
		if got.Rows != want.Rows || got.SHA256 != want.SHA256 {
			return report, fmt.Errorf("table %s content verification failed", want.Table)
		}
	}
	if destPG {
		if _, err := dest.ExecContext(ctx, "SET CONSTRAINTS ALL IMMEDIATE"); err != nil {
			return report, err
		}
		// Restore sequence high-water marks after preserving numeric/hidden row identities.
		for _, table := range tableNames() {
			for _, col := range []string{"id", "arc_rowid"} {
				if col == "id" && !slices.Contains(pgColumns[table], "id") {
					continue
				}
				if col == "arc_rowid" && !hasPortableRowID(table) {
					continue
				}
				var seq sql.NullString
				if err := dest.QueryRowContext(ctx, "SELECT pg_get_serial_sequence(?,?)", table, col).Scan(&seq); err != nil {
					// Not every table has either column; discover columns rather than ignoring SQL failures.
					return report, err
				}
				if seq.Valid {
					if _, err := dest.ExecContext(ctx, "SELECT setval(?::regclass,GREATEST(COALESCE((SELECT MAX("+quoted(col)+") FROM "+quoted(table)+"),1),COALESCE(pg_sequence_last_value(?::regclass),1),1),EXISTS(SELECT 1 FROM "+quoted(table)+") OR pg_sequence_last_value(?::regclass) IS NOT NULL)", seq.String, seq.String, seq.String); err != nil {
						return report, err
					}
				}
			}
		}
	} else {
		r, err := dest.QueryContext(ctx, "PRAGMA foreign_key_check")
		if err != nil {
			return report, err
		}
		bad := r.Next()
		err = r.Err()
		r.Close()
		if err != nil {
			return report, err
		}
		if bad {
			return report, errors.New("portable backup contains broken references")
		}
	}
	report.Verified = true
	return report, nil
}

func portableTextColumn(table, col string) bool { return pgTextColumns[table][col] }

func upgradedPortableBackup(ctx context.Context, path string) (string, func(), error) {
	if err := CheckBackup(ctx, path); err != nil {
		return "", nil, err
	}
	dir, err := os.MkdirTemp("", "arc-restore-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { os.RemoveAll(dir) }
	input, err := os.Open(path)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	defer input.Close()
	target := filepath.Join(dir, "restore.db")
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	_, err = io.Copy(out, input)
	closeErr := out.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		cleanup()
		return "", nil, err
	}
	st, err := Open(ctx, target, Options{})
	if err != nil {
		cleanup()
		return "", nil, err
	}
	if err = st.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	return target, cleanup, nil
}

// SnapshotSQLite reads a stopped source, including WAL, without migrating or mutating it.
func SnapshotSQLite(ctx context.Context, source, dest string) (err error) {
	if _, err = os.Stat(source); err != nil {
		return err
	}
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	f.Close()
	// VACUUM INTO requires the reserved target to be empty, which it is here.
	ok := false
	defer func() {
		if !ok {
			os.Remove(dest)
		}
	}()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(source)+"?mode=ro")
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err = db.ExecContext(ctx, "VACUUM INTO ?", dest); err != nil {
		return err
	}
	if err = CheckBackup(ctx, dest); err != nil {
		return err
	}
	f, err = os.OpenFile(dest, os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	err = f.Sync()
	f.Close()
	if err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		d, e := os.Open(filepath.Dir(dest))
		if e != nil {
			return e
		}
		err = d.Sync()
		d.Close()
		if err != nil {
			return err
		}
	}
	ok = true
	return nil
}
