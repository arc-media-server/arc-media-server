package perftest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// PostgresDatabase gives a parity test its own database. It only accepts the
// loopback fixture and role created by tools/ci/check-postgres.sh.
func PostgresDatabase(t testing.TB) (string, bool) {
	t.Helper()
	dsn := os.Getenv("ARC_TEST_POSTGRES_PARITY")
	if dsn == "" {
		return "", false
	}
	control, err := url.Parse(dsn)
	if err != nil || control.Scheme != "postgres" || control.Hostname() != "127.0.0.1" ||
		control.Port() == "" || control.User == nil || control.User.Username() != "arc_parity" ||
		control.Path != "/arc_parity_control" || control.RawQuery != "sslmode=disable" {
		t.Fatal("database parity requires the disposable PostgreSQL fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal("cannot reach the parity fixture")
	}
	defer admin.Close(ctx)
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	name := "arc_parity_" + hex.EncodeToString(suffix[:])
	id := pgx.Identifier{name}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+id); err != nil {
		t.Fatal("cannot create an isolated parity database", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		admin, err := pgx.Connect(ctx, dsn)
		if err != nil {
			t.Error("could not reconnect to clean up the parity database")
			return
		}
		defer admin.Close(ctx)
		if _, err := admin.Exec(ctx, "DROP DATABASE "+id); err != nil {
			t.Error("could not clean up the parity database", err)
		}
	})
	control.Path = "/" + name
	return control.String(), true
}
