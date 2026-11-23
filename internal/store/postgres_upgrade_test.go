package store

import (
	"archive/zip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestPostgresUpgradeRecovery(t *testing.T) {
	source, target := os.Getenv("ARC_TEST_POSTGRES_UPGRADE"), os.Getenv("ARC_TEST_POSTGRES_RESTORE")
	if source == "" && target == "" {
		t.Skip("set both PostgreSQL upgrade and restore fixture URLs")
	}
	if source == "" || target == "" || source == target {
		t.Fatal("upgrade and restore require distinct disposable PostgreSQL databases")
	}
	ctx := t.Context()
	previous, err := currentPostgresSchema()
	if err != nil {
		t.Fatal(err)
	}
	version := "zz_fixture_profile_note"
	addColumn := "ALTER TABLE users ADD COLUMN fixture_note text NOT NULL DEFAULT 'initial note'"
	makeSchema := func(statements ...string) postgresSchema {
		t.Helper()
		versions := append(slices.Clone(previous.versions), version)
		steps := append(slices.Clone(previous.steps), postgresStep{version: version, statements: statements})
		schema, err := newPostgresSchema(versions, steps)
		if err != nil {
			t.Fatal(err)
		}
		return schema
	}
	valid := makeSchema(addColumn)
	broken := makeSchema(addColumn, "UPDATE no_such_fixture_table SET ignored=1")
	open := func(dsn, backupDir string, schema postgresSchema) *Store {
		t.Helper()
		st, err := openPostgres(ctx, dsn, backupDir, schema)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { st.Close() })
		return st
	}
	expectFailure := func(backupDir string, schema postgresSchema, contains string) {
		t.Helper()
		st, err := openPostgres(ctx, source, backupDir, schema)
		if err == nil {
			st.Close()
			t.Fatal("unexpected PostgreSQL upgrade success")
		}
		if !strings.Contains(err.Error(), contains) {
			t.Fatalf("wanted %q failure, got %v", contains, err)
		}
	}
	st := open(source, "", previous)
	const profileName = "Fixture,\"member\"\nSecond line"
	u, err := st.CreateUser(ctx, profileName, true, "violet", "fixture-hash", "")
	if err != nil {
		t.Fatal(err)
	}
	identity, err := st.Setting(ctx, "server_id")
	if err != nil || identity == "" {
		t.Fatal("missing server identity", err)
	}
	playID, err := st.AddPlay(ctx, Play{UserID: u.ID, ItemID: "fixture-item", Kind: "movie", Title: "Fixture"})
	if err != nil || playID == 0 {
		t.Fatal("could not seed history", err)
	}
	// Leave a gap to prove restore preserves the sequence rather than just the largest row.
	if _, err := st.db.ExecContext(ctx, "SELECT setval('play_history_id_seq', 1000)"); err != nil {
		t.Fatal(err)
	}
	st.Close()

	badDir := filepath.Join(t.TempDir(), "file-instead-of-directory")
	if err := os.WriteFile(badDir, nil, 0600); err != nil {
		t.Fatal(err)
	}
	expectFailure(badDir, valid, "before-upgrade backup")
	failedUpgradeDir := t.TempDir()
	expectFailure(failedUpgradeDir, broken, "migration "+version)
	afterFailure := open(source, "", previous)
	var hasColumn bool
	if err := afterFailure.db.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM information_schema.columns WHERE table_schema='public'
		AND table_name='users' AND column_name='fixture_note')`).Scan(&hasColumn); err != nil || hasColumn {
		t.Fatal("failed upgrade left a column behind", err)
	}
	afterFailure.Close()
	failed, err := filepath.Glob(filepath.Join(failedUpgradeDir, "*-before-upgrade.pg.zip"))
	if err != nil || len(failed) != 1 {
		t.Fatal("failed upgrade did not retain a verified archive", err)
	}

	backupDir := filepath.Join(t.TempDir(), "backups")
	upgraded := open(source, backupDir, valid)
	var note string
	if err := upgraded.db.QueryRowContext(ctx, "SELECT fixture_note FROM users WHERE id=?", u.ID).Scan(&note); err != nil || note != "initial note" {
		t.Fatal("upgraded profile did not receive the new field", err)
	}
	upgraded.Close()
	open(source, backupDir, valid).Close()
	archives, err := filepath.Glob(filepath.Join(backupDir, "*-before-upgrade.pg.zip"))
	if err != nil || len(archives) != 1 {
		t.Fatalf("want one safety archive after upgrade and restart, got %d (%v)", len(archives), err)
	}
	damaged := postgresArchiveWithoutProfileRows(t, archives[0])
	if err := RestorePostgresBackup(ctx, target, damaged); err == nil || !strings.Contains(err.Error(), "row count") {
		t.Fatal("restore did not reject an incomplete archive", err)
	}
	// A successful restore to the same target demonstrates that the failed COPY
	// and any tables created before it were all rolled back.

	if err := RestorePostgresBackup(ctx, target, archives[0]); err != nil {
		t.Fatal(err)
	}
	if err := RestorePostgresBackup(ctx, target, archives[0]); err == nil || !strings.Contains(err.Error(), "empty database") {
		t.Fatal("restore didn't protect the populated target", err)
	}
	recovered := open(target, "", previous)
	gotIdentity, err := recovered.Setting(ctx, "server_id")
	if err != nil || gotIdentity != identity {
		t.Fatal("restore did not preserve server identity", err)
	}
	got, err := recovered.GetUser(ctx, u.ID)
	if err != nil || got == nil || got.Name != profileName || !got.IsAdmin {
		t.Fatal("restore did not preserve the profile and its CSV special characters", err)
	}
	plays, err := recovered.Plays(ctx, PlayFilter{UserID: u.ID})
	if err != nil || len(plays) != 1 || plays[0].ID != playID {
		t.Fatal("restore did not preserve watch history", err)
	}
	next, err := recovered.AddPlay(ctx, Play{UserID: u.ID, ItemID: "new-item", Kind: "movie", Title: "After restore"})
	if err != nil || next != 1001 {
		t.Fatalf("restored sequence: got %d, want 1001 (%v)", next, err)
	}
}

func postgresArchiveWithoutProfileRows(t *testing.T, source string) string {
	t.Helper()
	z, err := zip.OpenReader(source)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	manifest, err := postgresManifest(&z.Reader)
	if err != nil {
		t.Fatal(err)
	}
	index := slices.IndexFunc(manifest.Tables, func(table postgresTable) bool { return table.Name == "users" })
	if index == -1 {
		t.Fatal("backup is missing its profile table")
	}

	f, err := os.Create(filepath.Join(t.TempDir(), "damaged.pg.zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out := zip.NewWriter(f)
	for _, entry := range z.File {
		if entry.Name == postgresTableEntry(index) {
			if _, err := out.Create(entry.Name); err != nil {
				t.Fatal(err)
			}
		} else if err := out.Copy(entry); err != nil {
			t.Fatal(err)
		}
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}
