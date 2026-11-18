package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPostgresVersionedReleaseContracts(t *testing.T) {
	releases, err := postgresReleases()
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) < 2 {
		t.Fatal("initial release and its upgrade must remain available")
	}
	for _, r := range releases {
		if len(r.catalog) == 0 {
			t.Fatalf("release %d has no verified catalog", r.Version)
		}
		layout := releasePortableLayout(r)
		if !reflect.DeepEqual(layout.columns, pgColumns) && r.Version == releases[len(releases)-1].Version {
			t.Fatal("latest columns differ")
		}
		if !layout.rowID["items"] || !layout.text["items"]["title"] {
			t.Fatal("historical portable layout is incomplete")
		}
	}
}

// Make a real original-release database, including every persistent portable table.
func originalPostgres(t *testing.T, targets ...string) (string, string, TransferReport) {
	t.Helper()
	target := ""
	if len(targets) > 0 {
		target = targets[0]
	} else {
		target = postgresTarget(t)
	}
	source, root := richPortableSQLite(t)
	releases, err := postgresReleases()
	if err != nil {
		t.Fatal(err)
	}
	old := releases[0]
	db, done, err := postgresControl(t.Context(), target)
	if err != nil {
		t.Fatal(err)
	}
	defer done()
	tx, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(t.Context(), old.baseline); err != nil {
		t.Fatal(err)
	}
	src, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	report, err := copyPortableLayout(t.Context(), src, tx, false, true, releasePortableLayout(old))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(t.Context(), "INSERT INTO arc_postgres_schema(singleton,fingerprint,transfer_id,transfer_report) VALUES(1,?,'existing-transfer','existing-report')", old.fingerprint); err != nil {
		t.Fatal(err)
	}
	if err = verifyPostgresRelease(t.Context(), tx, old); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return target, root, report
}

func TestPostgresSchemaUpgradePreservesDataAndBackup(t *testing.T) {
	target, root, want := originalPostgres(t)
	safety := filepath.Join(root, "before-upgrade.db")
	backups := 0
	var logs bytes.Buffer
	opts := Options{Log: slog.New(slog.NewTextHandler(&logs, nil)), BeforeMigrate: func(st *Store) error { backups++; return st.BackupTo(t.Context(), safety) }}
	st, err := OpenPostgres(t.Context(), target, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range want.Tables {
		got, err := tableDigest(t.Context(), st.db, table.Table, true)
		if err != nil || got != table {
			t.Fatalf("data changed %s: %+v %v", table.Table, got, err)
		}
	}
	var transferID, report string
	if err = st.db.QueryRowContext(t.Context(), "SELECT transfer_id,transfer_report FROM arc_postgres_schema").Scan(&transferID, &report); err != nil || transferID != "existing-transfer" || report != "existing-report" {
		t.Fatal("transfer ownership changed", err)
	}
	if err = st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = OpenPostgres(t.Context(), target, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if strings.Count(logs.String(), "\n") != 2 || strings.Contains(logs.String(), "fixture-key") || strings.Contains(logs.String(), "postgres://") {
		t.Fatal("unexpected upgrade log volume/privacy", logs.String())
	}
	if backups != 1 {
		t.Fatal("backup repeated on compatible restart", backups)
	}
	if err = CheckBackup(t.Context(), safety); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(safety)
	if info.Mode().Perm()&0077 != 0 {
		t.Fatal("backup is not owner-only")
	}
	if err = st.SetSetting(t.Context(), "tmdb_api_key", "changed"); err != nil {
		t.Fatal(err)
	}
	if err = st.RestoreFrom(t.Context(), safety); err != nil {
		t.Fatal("pre-upgrade recovery failed", err)
	}
	key, err := st.Setting(t.Context(), "tmdb_api_key")
	if err != nil || key != "fixture-key-not-real" {
		t.Fatal("recovery lost settings", err)
	}
}

func TestPostgresSchemaUpgradeRequiresVerifiedBackup(t *testing.T) {
	for _, mode := range []string{"missing", "failed", "no-op", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			target, root, _ := originalPostgres(t)
			opts := Options{}
			switch mode {
			case "failed":
				opts.BeforeMigrate = func(*Store) error { return errors.New("fixture disk failure") }
			case "no-op":
				opts.BeforeMigrate = func(*Store) error { return nil }
			case "cancelled":
				opts.BeforeMigrate = func(st *Store) error {
					ctx, cancel := context.WithCancel(t.Context())
					cancel()
					return st.BackupTo(ctx, filepath.Join(root, "cancelled.db"))
				}
			}
			if st, err := OpenPostgres(t.Context(), target, opts); err == nil {
				st.Close()
				t.Fatal("unsafe upgrade accepted")
			}
			db, done, err := postgresControl(t.Context(), target)
			if err != nil {
				t.Fatal(err)
			}
			defer done()
			releases, _ := postgresReleases()
			if err = verifyPostgresRelease(t.Context(), db, releases[0]); err != nil {
				t.Fatal("failed upgrade changed schema", err)
			}
			for _, suffix := range []string{"", "-wal", "-shm"} {
				if _, err = os.Stat(filepath.Join(root, "cancelled.db") + suffix); err == nil {
					t.Fatal("incomplete backup retained")
				}
			}
		})
	}
}

func TestPostgresSchemaUpgradeRejectsDriftAndNewerVersions(t *testing.T) {
	for _, mode := range []string{"column", "index", "function", "history", "newer", "version"} {
		t.Run(mode, func(t *testing.T) {
			target, root, _ := originalPostgres(t)
			db, done, err := postgresControl(t.Context(), target)
			if err != nil {
				t.Fatal(err)
			}
			query := map[string]string{
				"column":   "ALTER TABLE items ADD COLUMN unexpected TEXT",
				"index":    "DROP INDEX sessions_user",
				"function": "CREATE OR REPLACE FUNCTION instr(text,text) RETURNS bigint LANGUAGE SQL IMMUTABLE STRICT PARALLEL SAFE AS $arc$ SELECT 0::bigint $arc$",
				"history":  "DELETE FROM schema_migrations WHERE version='0063_provider_lookup'",
				"newer":    "UPDATE arc_postgres_schema SET fingerprint='newer-build'",
				"version":  "ALTER TABLE arc_postgres_schema ADD COLUMN schema_version BIGINT DEFAULT 999",
			}[mode]
			if _, err = db.ExecContext(t.Context(), query); err != nil {
				t.Fatal(err)
			}
			done()
			called := false
			st, err := OpenPostgres(t.Context(), target, Options{BeforeMigrate: func(st *Store) error {
				called = true
				return st.BackupTo(t.Context(), filepath.Join(root, "unexpected.db"))
			}})
			if err == nil {
				st.Close()
				t.Fatal("unsupported schema accepted")
			}
			if called {
				t.Fatal("backup/upgrade started on unrecognised schema")
			}
		})
	}
}

func TestPostgresSchemaUpgradeExcludesOtherInstance(t *testing.T) {
	target, root, _ := originalPostgres(t)
	var secondErr error
	st, err := OpenPostgres(t.Context(), target, Options{BeforeMigrate: func(s *Store) error {
		other, err := OpenPostgres(t.Context(), target, Options{})
		secondErr = err
		if other != nil {
			other.Close()
		}
		return s.BackupTo(t.Context(), filepath.Join(root, "safety.db"))
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if secondErr == nil || !strings.Contains(secondErr.Error(), "already used") {
		t.Fatal("second instance admitted", secondErr)
	}
}

// Inject pending future migrations only into this store instance, never process-global state.
// A real extra column, a failure and cancellation exercise the same transactional runner.
func TestPostgresFutureSchemaUpgradeTransaction(t *testing.T) {
	for _, mode := range []string{"column", "failed", "cancelled", "verification"} {
		t.Run(mode, func(t *testing.T) {
			target := postgresTarget(t)
			st, err := OpenPostgres(t.Context(), target, Options{})
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			if err = st.SetSetting(t.Context(), "keep", "value"); err != nil {
				t.Fatal(err)
			}
			lib, err := st.SyncLibrary(t.Context(), "Fixture", "movies", "tmdb_aired", nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = st.db.ExecContext(t.Context(), "INSERT INTO items(id,library_id,kind,match_key,title,sort_title,added_at,updated_at) VALUES('kept-item',?,'movie','kept','Existing title','existing title',1,1)", lib); err != nil {
				t.Fatal(err)
			}
			releases, _ := postgresReleases()
			old := releases[len(releases)-1]
			next := old
			next.Version++
			next.fingerprint = "fixture-future"
			next.upgrade = "ALTER TABLE items ADD COLUMN fixture_future TEXT NOT NULL DEFAULT 'kept';"
			ctx := t.Context()
			if mode == "failed" {
				next.upgrade += " INSERT INTO deliberately_missing_table VALUES(1);"
			}
			if mode == "cancelled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 300*time.Millisecond)
				defer cancel()
				next.upgrade += " SELECT pg_sleep(2);"
			}
			// Obtain the expected catalog for the successful extra column, then roll it back.
			tx, err := st.db.BeginTx(t.Context(), nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = tx.ExecContext(t.Context(), "ALTER TABLE items ADD COLUMN fixture_future TEXT NOT NULL DEFAULT 'kept'"); err != nil {
				t.Fatal(err)
			}
			next.catalog, err = postgresCatalog(t.Context(), tx)
			tx.Rollback()
			if err != nil {
				t.Fatal(err)
			}
			if mode == "verification" {
				next.catalog = old.catalog
			}
			safety := filepath.Join(t.TempDir(), "future-safety.db")
			err = st.upgradePostgres(ctx, Options{BeforeMigrate: func(s *Store) error { return s.BackupTo(t.Context(), safety) }}, []postgresRelease{old, next}, 0)
			if mode == "column" {
				if err != nil {
					t.Fatal(err)
				}
				var got string
				if err = st.db.QueryRowContext(t.Context(), "SELECT fixture_future FROM items WHERE id='kept-item'").Scan(&got); err != nil || got != "kept" {
					t.Fatal(err)
				}
				if err = verifyPostgresRelease(t.Context(), st.db, next); err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil {
					t.Fatal("failed upgrade reported success")
				}
				if err = verifyPostgresRelease(t.Context(), st.db, old); err != nil {
					t.Fatal("schema did not roll back", err)
				}
			}
			key, err := st.Setting(t.Context(), "keep")
			if err != nil || key != "value" {
				t.Fatal("data changed", err)
			}
			if err = CheckBackup(t.Context(), safety); err != nil {
				t.Fatal("safety copy lost", err)
			}
		})
	}
}

func TestPostgresHistoricalBackupUsesOldColumns(t *testing.T) {
	target := postgresTarget(t)
	// This schema genuinely predates current columns/tables; deleting a version marker would
	// not test whether the backup accidentally selects new columns from an older database.
	oldPath := oldBackupThrough(t, 55)
	sqlite, err := sql.Open("sqlite", oldPath)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlite.Close()
	oldStore := &Store{db: &database{DB: sqlite, writer: sqlite}}
	baseline, columns, err := postgresBaseline(t.Context(), oldStore)
	if err != nil {
		t.Fatal(err)
	}
	release := postgresRelease{Version: 1, SQLiteMigrations: 55, baseline: baseline, columns: columns}
	layout := releasePortableLayout(release)
	db, done, err := postgresControl(t.Context(), target)
	if err != nil {
		t.Fatal(err)
	}
	defer done()
	if _, err = db.ExecContext(t.Context(), baseline); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	want, err := copyPortableLayout(t.Context(), sqlite, tx, false, true, layout)
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	pg := &Store{db: &database{DB: db, writer: db}, backend: "postgres", pgBackupRelease: &release}
	safety := filepath.Join(t.TempDir(), "old-shape.db")
	if err = pg.BackupTo(t.Context(), safety); err != nil {
		t.Fatal("backup queried future columns", err)
	}
	backup, err := sql.Open("sqlite", safety)
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range want.Tables {
		got, err := tableDigestLayout(t.Context(), backup, table.Table, false, layout)
		if err != nil || got != table {
			t.Fatal(table.Table, "data changed", got, err)
		}
	}
	backup.Close()
	upgraded, err := Open(t.Context(), safety, Options{})
	if err != nil {
		t.Fatal("old backup is not upgradeable", err)
	}
	defer upgraded.Close()
	var title string
	if err = upgraded.db.QueryRowContext(t.Context(), "SELECT title FROM items WHERE id='movie'").Scan(&title); err != nil || title != "My edited title" {
		t.Fatal("old user data lost", title, err)
	}
}

func TestPostgresUpgradeScriptsCannotEscapeTransaction(t *testing.T) {
	for _, body := range []string{"ALTER TABLE items ADD COLUMN x TEXT; COMMIT;", "ROLLBACK;", "CREATE INDEX CONCURRENTLY unsafe ON items(title);", "VACUUM;"} {
		if err := validatePostgresUpgradeSQL(body); err == nil {
			t.Fatal("nontransactional migration admitted", body)
		}
	}
	for _, body := range []string{"ALTER TABLE items ADD COLUMN x TEXT DEFAULT 'COMMIT';", "CREATE FUNCTION fixture() RETURNS void LANGUAGE plpgsql AS $fn$ BEGIN RETURN; END $fn$;"} {
		if err := validatePostgresUpgradeSQL(body); err != nil {
			t.Fatal("transactional migration rejected", err)
		}
	}
}

func TestPostgresOlderCommittedTransferCanRecoverBeforeUpgrade(t *testing.T) {
	target, root, report := originalPostgres(t)
	raw, _ := json.Marshal(report)
	db, done, err := postgresControl(t.Context(), target)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(t.Context(), "UPDATE arc_postgres_schema SET transfer_report=?", string(raw)); err != nil {
		t.Fatal(err)
	}
	done()
	got, complete, err := CompletedTransfer(t.Context(), filepath.Join(root, "source.db"), target, "existing-transfer")
	if err != nil || !complete || !reflect.DeepEqual(got, report) {
		t.Fatal("old transfer could not recover", complete, err)
	}
	st, err := OpenPostgres(t.Context(), target, Options{BeforeMigrate: func(s *Store) error { return s.BackupTo(t.Context(), filepath.Join(root, "recovery-upgrade.db")) }})
	if err != nil {
		t.Fatal(err)
	}
	st.Close()
}

func TestPostgresMultiplePendingUpgradesRollbackTogether(t *testing.T) {
	st := postgresFixture(t)
	releases, _ := postgresReleases()
	old := releases[len(releases)-1]
	next := old
	next.Version++
	next.fingerprint = "fixture-step-three"
	next.upgrade = "ALTER TABLE items ADD COLUMN fixture_step TEXT;"
	tx, err := st.db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(t.Context(), next.upgrade); err != nil {
		t.Fatal(err)
	}
	next.catalog, err = postgresCatalog(t.Context(), tx)
	tx.Rollback()
	if err != nil {
		t.Fatal(err)
	}
	last := next
	last.Version++
	last.fingerprint = "fixture-step-four"
	last.upgrade = "INSERT INTO missing_upgrade_table VALUES(1);"
	calls := 0
	err = st.upgradePostgres(t.Context(), Options{BeforeMigrate: func(s *Store) error { calls++; return s.BackupTo(t.Context(), filepath.Join(t.TempDir(), "safety.db")) }}, []postgresRelease{old, next, last}, 0)
	if err == nil || calls != 1 {
		t.Fatal("broken chain accepted or backed up per step", calls, err)
	}
	if err = verifyPostgresRelease(t.Context(), st.db, old); err != nil {
		t.Fatal("earlier migration was committed despite later failure", err)
	}
}

func TestPostgresDisconnectedUpgradeRollsBack(t *testing.T) {
	st := postgresFixture(t)
	releases, _ := postgresReleases()
	old := releases[len(releases)-1]
	next := old
	next.Version++
	next.fingerprint = "fixture-disconnect"
	next.upgrade = "ALTER TABLE items ADD COLUMN fixture_interrupted TEXT; SELECT pg_sleep(5);"
	var pid int
	if err := st.pgLock.QueryRowContext(t.Context(), "SELECT pg_backend_pid()").Scan(&pid); err != nil {
		t.Fatal(err)
	}
	control, done, err := postgresControl(t.Context(), os.Getenv("ARC_TEST_POSTGRES"))
	if err != nil {
		t.Fatal(err)
	}
	defer done()
	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Second)
	defer cancel()
	killed := make(chan error, 1)
	go func() {
		tick := time.NewTicker(10 * time.Millisecond)
		defer tick.Stop()
		for {
			var sleeping bool
			err := control.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=? AND wait_event='PgSleep')", pid).Scan(&sleeping)
			if err != nil {
				killed <- err
				return
			}
			if sleeping {
				var success bool
				err = control.QueryRowContext(ctx, "SELECT pg_terminate_backend(?)", pid).Scan(&success)
				if err == nil && !success {
					err = errors.New("fixture backend was not terminated")
				}
				killed <- err
				return
			}
			select {
			case <-ctx.Done():
				killed <- ctx.Err()
				return
			case <-tick.C:
			}
		}
	}()
	safety := filepath.Join(t.TempDir(), "interrupted.db")
	err = st.upgradePostgres(ctx, Options{BeforeMigrate: func(s *Store) error { return s.BackupTo(t.Context(), safety) }}, []postgresRelease{old, next}, 0)
	if err == nil {
		t.Fatal("disconnected upgrade reported success")
	}
	if err = <-killed; err != nil {
		t.Fatal("interruption fixture failed", err)
	}
	if err = verifyPostgresRelease(t.Context(), st.db, old); err != nil {
		t.Fatal("interruption did not roll back", err)
	}
	if err = CheckBackup(t.Context(), safety); err != nil {
		t.Fatal("interruption lost safety backup", err)
	}
}

func TestPostgresSchemaUpgradeWithOrdinarySchemaOwner(t *testing.T) {
	target := postgresTarget(t)
	u, err := url.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	schema := u.Query().Get("search_path")
	role := "arc_upgrade_owner_" + NewID()
	control, done, err := postgresControl(t.Context(), target)
	if err != nil {
		t.Fatal(err)
	}
	defer done()
	if _, err = control.ExecContext(t.Context(), "CREATE ROLE "+quoted(role)+" LOGIN"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, closeCleanup, e := postgresControl(context.Background(), os.Getenv("ARC_TEST_POSTGRES"))
		if e != nil {
			t.Error(e)
			return
		}
		defer closeCleanup()
		if _, e = cleanup.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS "+quoted(schema)+" CASCADE"); e != nil {
			t.Error(e)
		}
		if _, e = cleanup.ExecContext(context.Background(), "DROP ROLE "+quoted(role)); e != nil {
			t.Error(e)
		}
	})
	// The fixture role owns only this namespace, with no superuser/database/role creation rights.
	if _, err = control.ExecContext(t.Context(), "ALTER SCHEMA "+quoted(schema)+" OWNER TO "+quoted(role)); err != nil {
		t.Fatal(err)
	}
	u.User = url.User(role) // fixture uses local trust auth; no real secret is needed
	target, root, _ := originalPostgres(t, u.String())
	st, err := OpenPostgres(t.Context(), target, Options{BeforeMigrate: func(s *Store) error { return s.BackupTo(t.Context(), filepath.Join(root, "ordinary-owner.db")) }})
	if err != nil {
		t.Fatal(err)
	}
	if err = st.Close(); err != nil {
		t.Fatal(err)
	}
}
