package store

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestOptimizeDatabaseSpaceGatesAndRealVacuum(t *testing.T) {
	for _, mode := range []string{"enough space", "low space", "unknown space", "busy", "cancelled", "small freelist"} {
		t.Run(mode, func(t *testing.T) {
			st := openTest(t)
			ctx := t.Context()
			if _, err := st.db.ExecContext(ctx, "CREATE TABLE waste (data BLOB)"); err != nil {
				t.Fatal(err)
			}
			for range 8 {
				if _, err := st.db.ExecContext(ctx, "INSERT INTO waste VALUES (zeroblob(262144))"); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := st.db.ExecContext(ctx, "DELETE FROM waste"); err != nil {
				t.Fatal(err)
			}
			conn, err := st.db.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := conn.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
				t.Fatal(err)
			}
			conn.Close()
			allowed := func() bool { return mode != "busy" }
			available := func(string) (uint64, error) {
				if mode == "unknown space" {
					return 0, errors.New("unavailable")
				}
				if mode == "low space" {
					return 1, nil
				}
				return 1 << 40, nil
			}
			if mode == "cancelled" {
				c, cancel := context.WithCancel(ctx)
				cancel()
				ctx = c
			}
			minimum := int64(4096)
			if mode == "small freelist" {
				minimum = 1 << 40
			}
			result, err := st.optimizeDatabase(ctx, allowed, available, minimum)
			if mode == "busy" || mode == "cancelled" {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("gate=%v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if mode == "enough space" {
				if !result.Vacuumed || result.AfterBytes >= result.BeforeBytes {
					t.Fatalf("no real compaction: %+v", result)
				}
			} else if result.Vacuumed || result.VacuumSkipped == "" {
				t.Fatalf("unsafe vacuum: %+v", result)
			}
			var n int
			if err := st.db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM schema_migrations").Scan(&n); err != nil || n == 0 {
				t.Fatalf("schema lost: %d %v", n, err)
			}
		})
	}
}

func TestOptimizeDatabaseReportsCorruptIntegrity(t *testing.T) {
	st := openTest(t)
	ctx := t.Context()
	conn, err := st.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"CREATE TABLE damage(v TEXT)", "INSERT INTO damage VALUES ('retained')", "CREATE INDEX damage_v ON damage(v)", "PRAGMA writable_schema=ON", "UPDATE sqlite_schema SET rootpage=999999 WHERE name='damage_v'", "PRAGMA writable_schema=RESET"} {
		if _, err := conn.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	err = checkIntegrity(ctx, conn)
	conn.Close()
	if !errors.Is(err, ErrIntegrity) {
		t.Fatalf("corruption hidden: %v", err)
	}
}

func TestMaintenanceResultPersists(t *testing.T) {
	st := openTest(t)
	want := MaintenanceRun{StartedAt: 100, FinishedAt: 200, LastSuccessAt: 90, Detail: "Compaction refused", Error: "not enough space"}
	if err := st.SaveMaintenanceRun(t.Context(), "database", want); err != nil {
		t.Fatal(err)
	}
	got, err := st.MaintenanceRun(t.Context(), "database")
	if err != nil || got != want {
		t.Fatalf("result=%+v err=%v", got, err)
	}
	if !strings.Contains(got.Error, "space") {
		t.Fatal("failure lost")
	}
}
