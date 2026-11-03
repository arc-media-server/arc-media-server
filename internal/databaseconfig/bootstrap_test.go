package databaseconfig

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"ams/internal/store"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func freshTarget(t *testing.T) Connection {
	t.Helper()
	dsn := os.Getenv("ARC_TEST_POSTGRES")
	if dsn == "" {
		t.Skip("real PostgreSQL fixture not configured")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	name := "arc_cutover_" + store.NewID()
	if _, err = db.ExecContext(t.Context(), `CREATE DATABASE "`+name+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, e := sql.Open("pgx", dsn)
		if e == nil {
			defer cleanup.Close()
			cleanup.ExecContext(context.Background(), `DROP DATABASE "`+name+`" WITH (FORCE)`)
		}
	})
	port, _ := strconv.Atoi(u.Port())
	password, _ := u.User.Password()
	return Connection{Host: u.Hostname(), Port: port, Database: name, Username: u.User.Username(), Password: password, TLSMode: "disable"}
}
func sqliteInstance(t *testing.T) (*Manager, string) {
	t.Helper()
	dir := t.TempDir()
	m := New(dir, nil)
	st, err := m.Open(t.Context(), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = st.CreateUser(t.Context(), "Owner", true, "violet", "fixture-hash", ""); err != nil {
		t.Fatal(err)
	}
	if err = st.SetSetting(t.Context(), "fixture-secret", "must-remain-private"); err != nil {
		t.Fatal(err)
	}
	st.Close()
	return m, dir
}
func TestGuidedCutoverPreservesInstanceAndSecrets(t *testing.T) {
	c := freshTarget(t)
	m, dir := sqliteInstance(t)
	check, err := m.Test(t.Context(), c)
	if err != nil || !check.Empty || !check.CanCreate {
		t.Fatalf("preflight %+v %v", check, err)
	}
	if err = m.Request(t.Context(), c); err != nil {
		t.Fatal(err)
	}
	if !m.Transitioning() {
		t.Fatal("new requests were not gated")
	}
	<-m.Requested()
	status, err := m.Status()
	if err != nil || status.State != "requested" {
		t.Fatal(status, err)
	}
	raw, _ := json.Marshal(status)
	if strings.Contains(string(raw), c.Password) || strings.Contains(string(raw), "password") {
		t.Fatal("status exposed credentials")
	}
	// A new manager models the controlled server restart.
	next := New(dir, nil)
	if err = next.Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
	status, err = next.Status()
	if err != nil || status.Backend != "postgres" || status.LastReport == nil || !status.LastReport.Verified {
		t.Fatal(status, err)
	}
	st, err := next.Open(t.Context(), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	users, err := st.ListUsers(t.Context())
	if err != nil || len(users) != 1 || users[0].PasswordHash != "fixture-hash" {
		t.Fatal("profiles not preserved", err)
	}
	v, err := st.Setting(t.Context(), "fixture-secret")
	if err != nil || v != "must-remain-private" {
		t.Fatal("settings lost", err)
	}
	info, err := os.Stat(filepath.Join(dir, bootstrapName))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("unsafe credential-file permissions", err)
	}
	if err = next.Cancel(); err == nil {
		t.Fatal("active PostgreSQL reverted to stale SQLite")
	}
	if err = os.Rename(filepath.Join(dir, bootstrapName), filepath.Join(dir, "retained-bootstrap")); err != nil {
		t.Fatal(err)
	}
	if _, err = next.Open(t.Context(), store.Options{}); err == nil {
		t.Fatal("missing selector silently fell back to SQLite")
	}
}
func TestCommittedTargetRecoversBeforeSelectorSwitch(t *testing.T) {
	c := freshTarget(t)
	m, dir := sqliteInstance(t)
	if err := m.Request(t.Context(), c); err != nil {
		t.Fatal(err)
	}
	b, err := m.load()
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(dir, "backups"), 0700)
	b.Pending.Snapshot = "ams-20261103-120000-before-upgrade.db"
	b.Pending.Phase = "copying"
	snapshot := filepath.Join(dir, "backups", b.Pending.Snapshot)
	if err = store.SnapshotSQLite(t.Context(), filepath.Join(dir, "ams.db"), snapshot); err != nil {
		t.Fatal(err)
	}
	if err = m.save(b); err != nil {
		t.Fatal(err)
	}
	dsn, _ := c.DSN()
	if _, err = store.TransferSQLiteWithID(t.Context(), snapshot, dsn, b.Pending.ID); err != nil {
		t.Fatal(err)
	}
	// Target commit happened, but neither 'verified' journal nor active selector was saved.
	if err = New(dir, nil).Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
	b, err = m.load()
	if err != nil || b.Backend != "postgres" || b.Pending != nil {
		t.Fatal("ambiguous cutover", err)
	}
}
func TestFailureBeforeCutoverNeverChangesSourceSelector(t *testing.T) {
	c := freshTarget(t)
	m, dir := sqliteInstance(t)
	if err := m.Request(t.Context(), c); err != nil {
		t.Fatal(err)
	}
	b, err := m.load()
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a target configuration/network failure after initial preflight.
	b.Pending.Target.Port = 1
	if err = m.save(b); err != nil {
		t.Fatal(err)
	}
	if err = New(dir, nil).Recover(t.Context()); err == nil {
		t.Fatal("unreachable target succeeded")
	}
	b, err = m.load()
	if err != nil || b.Backend != "sqlite" || b.Pending == nil {
		t.Fatal("source selector changed", err)
	}
	if err = m.Cancel(); err != nil {
		t.Fatal(err)
	}
	st, err := m.Open(t.Context(), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	v, err := st.Setting(t.Context(), "fixture-secret")
	if err != nil || v != "must-remain-private" {
		t.Fatal("source lost", err)
	}
}
func TestTLSIsVerifiedByDefaultAndNeverDowngrades(t *testing.T) {
	c := freshTarget(t)
	c.TLSMode = ""
	dsn, err := c.DSN()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dsn, "sslmode=verify-full") {
		t.Fatal("TLS verification not default")
	}
	if _, err = store.InspectPostgres(t.Context(), dsn); err == nil {
		t.Fatal("unencrypted fixture accepted verified TLS")
	}
	if strings.Contains(err.Error(), c.Password) || strings.Contains(err.Error(), dsn) {
		t.Fatal("failed connection leaked secrets")
	}
}
func TestBootstrapRejectsUnsafePermissionsAndSymlinks(t *testing.T) {
	m, dir := sqliteInstance(t)
	path := filepath.Join(dir, bootstrapName)
	if err := os.WriteFile(path, []byte(`{"version":1,"backend":"sqlite"}`), 0644); err != nil {
		t.Fatal(err)
	}
	os.Chmod(path, 0644)
	if _, err := m.Status(); err == nil {
		t.Fatal("world-readable credential bootstrap accepted")
	}
	os.Remove(path)
	if err := os.Symlink("ams.db", path); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Status(); err == nil {
		t.Fatal("symlink bootstrap accepted")
	}
}

func TestPostgresWorksWithOrdinaryDatabaseOwner(t *testing.T) {
	c := freshTarget(t)
	adminDSN, _ := c.DSN()
	admin, err := sql.Open("pgx", adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	role := "arc_role_" + store.NewID()
	if _, err = admin.ExecContext(t.Context(), `CREATE ROLE "`+role+`" LOGIN PASSWORD 'fixture-role-only' NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION`); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.ExecContext(t.Context(), `ALTER DATABASE "`+c.Database+`" OWNER TO "`+role+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		dsn := os.Getenv("ARC_TEST_POSTGRES")
		db, e := sql.Open("pgx", dsn)
		if e == nil {
			defer db.Close()
			db.ExecContext(context.Background(), `DROP DATABASE "`+c.Database+`" WITH (FORCE)`)
			db.ExecContext(context.Background(), `DROP ROLE "`+role+`"`)
		}
	})
	c.Username = role
	c.Password = "fixture-role-only"
	m, _ := sqliteInstance(t)
	if err = m.Request(t.Context(), c); err != nil {
		t.Fatal(err)
	}
	if err = m.Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
	st, err := m.Open(t.Context(), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err = st.OptimizeDatabase(t.Context(), func() bool { return true }); err != nil {
		t.Fatal("ordinary owner cannot maintain own tables", err)
	}
	path := filepath.Join(t.TempDir(), "role-backup.db")
	if err = st.BackupTo(t.Context(), path); err != nil {
		t.Fatal(err)
	}
	if err = st.RestoreFrom(t.Context(), path); err != nil {
		t.Fatal(err)
	}
}
