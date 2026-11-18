package store

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
)

// These snapshots are immutable release contracts, not replacements for the latest baseline.
//
//go:embed postgres/releases
var pgReleasesFS embed.FS

const postgresSchemaVersion = 2

type postgresRelease struct {
	Version          int `json:"version"`
	SQLiteMigrations int `json:"sqlite_migrations"`
	baseline         string
	columns          map[string][]string
	catalog          []pgCatalogEntry
	fingerprint      string
	upgrade          string
}

type pgCatalogEntry struct {
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Definition string `json:"definition"`
}

func postgresReleases() ([]postgresRelease, error) {
	dirs, err := fs.ReadDir(pgReleasesFS, "postgres/releases")
	if err != nil {
		return nil, err
	}
	var releases []postgresRelease
	names, err := migrationNames()
	if err != nil {
		return nil, err
	}
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		base := "postgres/releases/" + dir.Name() + "/"
		b, err := pgReleasesFS.ReadFile(base + "manifest.json")
		if err != nil {
			return nil, err
		}
		var r postgresRelease
		if err = json.Unmarshal(b, &r); err != nil {
			return nil, err
		}
		if r.Version != len(releases)+1 || r.SQLiteMigrations < 1 || r.SQLiteMigrations > len(names) {
			return nil, errors.New("invalid PostgreSQL release sequence")
		}
		b, err = pgReleasesFS.ReadFile(base + "baseline.sql")
		if err != nil {
			return nil, err
		}
		r.baseline = string(b)
		if !strings.Contains(r.baseline, "-- Arc migration fingerprint: "+migrationFingerprintThrough(r.SQLiteMigrations)) {
			return nil, errors.New("historical SQLite migrations changed; PostgreSQL release no longer matches")
		}
		cols, err := pgReleasesFS.ReadFile(base + "tables.json")
		if err != nil {
			return nil, err
		}
		if err = json.Unmarshal(cols, &r.columns); err != nil {
			return nil, err
		}
		h := sha256.New()
		h.Write(b)
		h.Write(cols)
		r.fingerprint = hex.EncodeToString(h.Sum(nil))
		// The catalog generation tool starts from baseline SQL and fills this file explicitly.
		catalog, err := pgReleasesFS.ReadFile(base + "catalog.json")
		if err == nil {
			if err = json.Unmarshal(catalog, &r.catalog); err != nil {
				return nil, err
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		if r.Version > 1 {
			upgrade, err := pgReleasesFS.ReadFile(base + "upgrade.sql")
			if err != nil {
				return nil, err
			}
			r.upgrade = string(upgrade)
			if err = validatePostgresUpgradeSQL(r.upgrade); err != nil {
				return nil, err
			}
			if strings.TrimSpace(r.upgrade) == "" {
				return nil, errors.New("PostgreSQL upgrade script is empty")
			}
			if r.SQLiteMigrations < releases[len(releases)-1].SQLiteMigrations {
				return nil, errors.New("PostgreSQL release rewinds SQLite migrations")
			}
		}
		releases = append(releases, r)
	}
	if len(releases) == 0 || releases[len(releases)-1].Version != postgresSchemaVersion || releases[len(releases)-1].fingerprint != postgresFingerprint() || releases[len(releases)-1].SQLiteMigrations != len(names) {
		return nil, errors.New("PostgreSQL baseline changed without a versioned release and upgrade path")
	}
	return releases, nil
}

func migrationFingerprintThrough(count int) string {
	h := sha256.New()
	names, _ := migrationNames()
	for _, name := range names[:count] {
		body, _ := migrationsFS.ReadFile(name)
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write(body)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// The deparsed catalog deliberately excludes PG18's separately represented NOT NULL constraints;
// attnotnull records those on every supported major. No row values, schema names or secrets appear.
func postgresCatalog(ctx context.Context, db querySource) ([]pgCatalogEntry, error) {
	queries := []struct{ kind, sql string }{
		{"relation", `SELECT c.relname,c.relkind::text||':'||c.relrowsecurity::text||':'||c.relforcerowsecurity::text FROM pg_class c WHERE c.relnamespace=current_schema()::regnamespace AND c.relkind IN ('r','p','v','m','f','S')`},
		{"column", `SELECT c.relname||'.'||a.attname, a.attnum::text||':'||format_type(a.atttypid,a.atttypmod)||':'||a.attnotnull::text||':'||a.attidentity::text||':'||COALESCE((SELECT collname FROM pg_collation WHERE oid=a.attcollation),'')||':'||COALESCE(pg_get_expr(d.adbin,d.adrelid),'') FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum WHERE c.relnamespace=current_schema()::regnamespace AND c.relkind IN ('r','p','v','m','f') AND a.attnum>0 AND NOT a.attisdropped`},
		{"constraint", `SELECT c.relname||'.'||p.conname,pg_get_constraintdef(p.oid) FROM pg_constraint p JOIN pg_class c ON c.oid=p.conrelid WHERE c.relnamespace=current_schema()::regnamespace AND p.contype IN ('p','u','f','c','x')`},
		{"index", `SELECT c.relname,pg_get_indexdef(c.oid) FROM pg_class c WHERE c.relnamespace=current_schema()::regnamespace AND c.relkind IN ('i','I')`},
		{"function", `SELECT p.proname||'('||pg_get_function_identity_arguments(p.oid)||')', pg_get_functiondef(p.oid) FROM pg_proc p WHERE p.pronamespace=current_schema()::regnamespace AND p.prokind='f'`},
		{"sequence", `SELECT c.relname,format_type(s.seqtypid,NULL)||':'||s.seqstart::text||':'||s.seqincrement::text||':'||s.seqmin::text||':'||s.seqmax::text||':'||s.seqcache::text||':'||s.seqcycle::text FROM pg_sequence s JOIN pg_class c ON c.oid=s.seqrelid WHERE c.relnamespace=current_schema()::regnamespace`},
		{"operator", `SELECT o.oprname||'('||o.oprleft::regtype::text||','||o.oprright::regtype::text||')',p.proname||':'||o.oprresult::regtype::text FROM pg_operator o JOIN pg_proc p ON p.oid=o.oprcode WHERE o.oprnamespace=current_schema()::regnamespace`},
		{"sequence_owner", `SELECT c.relname,t.relname||'.'||a.attname FROM pg_class c JOIN pg_depend d ON d.objid=c.oid AND d.classid='pg_class'::regclass JOIN pg_class t ON t.oid=d.refobjid JOIN pg_attribute a ON a.attrelid=t.oid AND a.attnum=d.refobjsubid WHERE c.relnamespace=current_schema()::regnamespace AND c.relkind='S' AND d.deptype IN ('a','i')`},
		{"trigger", `SELECT c.relname||'.'||t.tgname,pg_get_triggerdef(t.oid) FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid WHERE c.relnamespace=current_schema()::regnamespace AND NOT t.tgisinternal`},
	}
	var schema string
	rows, err := db.QueryContext(ctx, "SELECT current_schema()")
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		rows.Close()
		return nil, errors.New("PostgreSQL schema is unavailable")
	}
	err = rows.Scan(&schema)
	rows.Close()
	if err != nil {
		return nil, err
	}
	var out []pgCatalogEntry
	for _, q := range queries {
		rows, err := db.QueryContext(ctx, q.sql)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var e pgCatalogEntry
			e.Kind = q.kind
			if err = rows.Scan(&e.Name, &e.Definition); err != nil {
				rows.Close()
				return nil, err
			}
			e.Definition = strings.ReplaceAll(e.Definition, quoted(schema)+".", "")
			e.Definition = strings.ReplaceAll(e.Definition, schema+".", "")
			out = append(out, e)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func verifyPostgresRelease(ctx context.Context, db querySource, r postgresRelease) error {
	var fingerprint string
	rowsMarker, err := db.QueryContext(ctx, "SELECT fingerprint FROM arc_postgres_schema WHERE singleton=1")
	if err != nil {
		return err
	}
	if !rowsMarker.Next() {
		rowsMarker.Close()
		return errors.New("PostgreSQL release marker is missing")
	}
	err = rowsMarker.Scan(&fingerprint)
	rowsMarker.Close()
	if err != nil {
		return err
	}
	if fingerprint != r.fingerprint {
		return errors.New("PostgreSQL release marker differs from its recognised schema")
	}
	if len(r.catalog) == 0 {
		return errors.New("PostgreSQL release catalog is missing; upgrade refused")
	}
	actual, err := postgresCatalog(ctx, db)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(actual, r.catalog) {
		return errors.New("PostgreSQL structure differs from the recognised Arc schema; restore a verified backup or use the matching build")
	}
	rows, err := db.QueryContext(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return err
	}
	var applied []string
	for rows.Next() {
		var v string
		if err = rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		applied = append(applied, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	names, _ := migrationNames()
	want := make([]string, r.SQLiteMigrations)
	for i, n := range names[:r.SQLiteMigrations] {
		want[i] = migrationVersion(n)
	}
	if !reflect.DeepEqual(applied, want) {
		return errors.New("PostgreSQL migration history is newer, incomplete or unsupported")
	}
	if r.Version >= 2 {
		var version int
		rows, err := db.QueryContext(ctx, "SELECT schema_version FROM arc_postgres_schema WHERE singleton=1")
		if err != nil {
			return err
		}
		if !rows.Next() {
			rows.Close()
			return errors.New("PostgreSQL version marker is missing")
		}
		err = rows.Scan(&version)
		rows.Close()
		if err != nil {
			return err
		}
		if version != r.Version {
			return errors.New("PostgreSQL schema version differs from its release marker")
		}
	}
	return nil
}

func (s *Store) upgradePostgres(ctx context.Context, opts Options, releases []postgresRelease, index int) (err error) {
	old := releases[index]
	if err := verifyPostgresRelease(ctx, s.db, old); err != nil {
		return err
	}
	if index == len(releases)-1 {
		return nil
	}
	if opts.BeforeMigrate == nil {
		return errors.New("PostgreSQL upgrade requires a verified before-upgrade backup; configure the startup backup hook")
	}
	started := time.Now()
	if opts.Log != nil {
		opts.Log.Info("PostgreSQL schema upgrade started", "from_version", old.Version, "to_version", releases[len(releases)-1].Version)
	}
	defer func() {
		if err == nil && opts.Log != nil {
			opts.Log.Info("PostgreSQL schema upgrade completed", "from_version", releases[index].Version, "to_version", releases[len(releases)-1].Version, "took", time.Since(started).Round(time.Millisecond))
		}
	}()
	s.pgBackupRelease = &old
	s.pgUpgradeBackupVerified = false
	err = opts.BeforeMigrate(s)
	s.pgBackupRelease = nil
	if err != nil {
		return fmt.Errorf("PostgreSQL pre-upgrade backup failed: %w", err)
	}
	if !s.pgUpgradeBackupVerified {
		return errors.New("PostgreSQL pre-upgrade hook did not create a verified portable backup")
	}
	tx, err := s.pgLock.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Startup owns the existing session advisory lock. Bound waits on unexpected external DDL.
	if _, err = tx.ExecContext(ctx, "SET LOCAL lock_timeout='5s'; SET LOCAL statement_timeout='0'"); err != nil {
		return err
	}
	if err = verifyPostgresRelease(ctx, tx, old); err != nil {
		return err
	}
	names, _ := migrationNames()
	for _, next := range releases[index+1:] {
		if err = validatePostgresUpgradeSQL(next.upgrade); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, next.upgrade); err != nil {
			return fmt.Errorf("PostgreSQL schema upgrade %d failed; transaction rolled back, safety backup retained: %w", next.Version, err)
		}
		for _, name := range names[old.SQLiteMigrations:next.SQLiteMigrations] {
			if _, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations(version,applied_at) VALUES(?,?)", migrationVersion(name), time.Now().Unix()); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, "UPDATE arc_postgres_schema SET fingerprint=?,schema_version=? WHERE singleton=1", next.fingerprint, next.Version); err != nil {
			return err
		}
		if err = verifyPostgresRelease(ctx, tx, next); err != nil {
			return fmt.Errorf("PostgreSQL schema upgrade verification failed; safety backup retained: %w", err)
		}
		old = next
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return tx.Commit()
}

// GeneratePostgresReleaseCatalogs is a developer tool: create/drop uniquely named fixture schemas
// in an explicitly supplied test database. It never inspects or changes existing Arc schemas.
func GeneratePostgresReleaseCatalogs(ctx context.Context, dsn, dir string) error {
	releases, err := postgresReleases()
	if err != nil {
		return err
	}
	db, done, err := postgresControl(ctx, dsn)
	if err != nil {
		return err
	}
	defer done()
	for _, release := range releases {
		schema := "arc_catalog_" + NewID()
		if _, err = db.ExecContext(ctx, "CREATE SCHEMA "+quoted(schema)); err != nil {
			return err
		}
		runErr := func() error {
			defer db.ExecContext(context.Background(), "DROP SCHEMA "+quoted(schema)+" CASCADE")
			if _, err := db.ExecContext(ctx, "SET search_path TO "+quoted(schema)); err != nil {
				return err
			}
			if _, err := db.ExecContext(ctx, release.baseline); err != nil {
				return err
			}
			catalog, err := postgresCatalog(ctx, db)
			if err != nil {
				return err
			}
			b, err := json.MarshalIndent(catalog, "", "  ")
			if err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(dir, fmt.Sprintf("%04d", release.Version), "catalog.json"), append(b, '\n'), 0644)
		}()
		if runErr != nil {
			return runErr
		}
	}
	return nil
}

// A migration must not end or escape the runner's transaction. Dollar-quoted function bodies
// are single SQL values; transaction-like words inside those bodies are not top-level commands.
func validatePostgresUpgradeSQL(body string) error {
	tokens := significantSQL(body)
	first := true
	blocked := map[string]bool{"BEGIN": true, "START": true, "COMMIT": true, "END": true, "ROLLBACK": true, "ABORT": true, "PREPARE": true, "SAVEPOINT": true, "RELEASE": true, "VACUUM": true, "DISCARD": true}
	for i := 0; i < len(tokens); i++ {
		if tokens[i] == "$" && i+1 < len(tokens) {
			width := 0
			if tokens[i+1] == "$" {
				width = 2
			} else if i+2 < len(tokens) && tokens[i+2] == "$" {
				width = 3
			}
			if width > 0 {
				end := -1
				for j := i + width; j+width <= len(tokens); j++ {
					if reflect.DeepEqual(tokens[i:i+width], tokens[j:j+width]) {
						end = j + width - 1
						break
					}
				}
				if end < 0 {
					return errors.New("unterminated PostgreSQL migration function body")
				}
				i = end
				first = false
				continue
			}
		}
		upper := strings.ToUpper(tokens[i])
		if (first && blocked[upper]) || upper == "CONCURRENTLY" {
			return errors.New("PostgreSQL startup migrations must remain transactional; transaction control and concurrent maintenance are unsupported")
		}
		first = tokens[i] == ";"
	}
	return nil
}
