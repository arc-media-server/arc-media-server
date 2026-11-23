package store

import (
	"testing"

	"ams/internal/perftest"
)

func TestVerifyPostgresConnectionRejectsDatabaseCopy(t *testing.T) {
	dsn, ok := perftest.PostgresDatabase(t)
	if !ok {
		t.Skip("requires the disposable PostgreSQL fixture")
	}
	copyDSN, _ := perftest.PostgresDatabase(t)
	ctx := t.Context()
	id := NewID()
	active, err := OpenSelectedPostgres(ctx, dsn, "", id, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer active.Close()
	copyStore, err := OpenSelectedPostgres(ctx, copyDSN, "", id, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer copyStore.Close()

	if err := active.VerifyPostgresConnection(ctx, dsn, id); err != nil {
		t.Fatalf("current database rejected: %v", err)
	}
	if err := active.VerifyPostgresConnection(ctx, copyDSN, id); err == nil {
		t.Fatal("a copy with the same Arc identity was accepted")
	}
}
