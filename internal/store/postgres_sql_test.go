package store

import (
	"context"
	"database/sql/driver"
	"fmt"
	"strings"
	"testing"
)

func TestPostgresBindingsPreserveLiteralsAndRepeatedSlots(t *testing.T) {
	q, a := postgresSQL("SELECT '?1', ?2, ?2, ?1 -- ?3\n", []driver.NamedValue{{Ordinal: 1, Value: "first"}, {Ordinal: 2, Value: "second"}})
	if !strings.Contains(q, "'?1'") || strings.Count(q, "$1") != 2 || len(a) != 2 || a[0].Value != "second" || a[1].Value != "first" {
		t.Fatal(q, a)
	}
	q, a = postgresSQL("DELETE FROM items WHERE id=?2", []driver.NamedValue{{Ordinal: 1, Value: "unused"}, {Ordinal: 2, Value: "used"}})
	if strings.Contains(q, "$2") || len(a) != 1 || a[0].Value != "used" {
		t.Fatal(q, a)
	}
}
func TestPostgresErrorsRedactWrappedConnectionDetails(t *testing.T) {
	err := fmt.Errorf("connection contains password=private: %w", context.DeadlineExceeded)
	if got := safePGError(err); got != context.DeadlineExceeded {
		t.Fatalf("got a wrapped driver error instead of the safe deadline sentinel")
	}
}
