package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type portableLayout struct {
	columns    map[string][]string
	text       map[string]map[string]bool
	rowID      map[string]bool
	migrations int
}

func currentPortableLayout() portableLayout {
	names, _ := migrationNames()
	return portableLayout{pgColumns, pgTextColumns, map[string]bool{"items": true, "item_genres": true, "item_labels": true}, len(names)}
}
func releasePortableLayout(r postgresRelease) portableLayout {
	layout := portableLayout{columns: r.columns, text: map[string]map[string]bool{}, rowID: map[string]bool{}, migrations: r.SQLiteMigrations}
	for table, cols := range r.columns {
		start := strings.Index(r.baseline, "CREATE TABLE "+quoted(table)+" (")
		if start < 0 {
			continue
		}
		part := r.baseline[start:]
		end := strings.Index(part, "\n);")
		if end < 0 {
			continue
		}
		part = part[:end]
		layout.rowID[table] = strings.Contains(part, `"arc_rowid"`) || strings.Contains(part, "arc_rowid BIGINT")
		layout.text[table] = map[string]bool{}
		for _, col := range cols {
			layout.text[table][col] = strings.Contains(part, quoted(col)+" TEXT ")
		}
	}
	return layout
}
func (l portableLayout) tableNames() []string {
	names := make([]string, 0, len(l.columns))
	for name := range l.columns {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func initializePortableSchema(ctx context.Context, path string, count int) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err = db.ExecContext(ctx, `CREATE TABLE schema_migrations(version TEXT PRIMARY KEY, applied_at INTEGER NOT NULL)`); err != nil {
		return err
	}
	names, _ := migrationNames()
	for _, name := range names[:count] {
		body, err := migrationsFS.ReadFile(name)
		if err != nil {
			return err
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, string(body)); err == nil {
			_, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations VALUES(?,?)", migrationVersion(name), time.Now().Unix())
		}
		if err != nil {
			tx.Rollback()
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
func syncPortableFile(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	err = f.Sync()
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
