package store

import (
	"context"
	"path/filepath"
	"testing"
)

// Deleting a library cascades through every table that refers to its items and files. A
// referring column without an index makes each deleted row read that whole table: removing a
// large TV library took minutes. Every foreign key column must lead some index.
func TestForeignKeysAreIndexed(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "a.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rows, err := s.db.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='table'`)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, n)
	}
	rows.Close()
	for _, tb := range tables {
		fk, err := s.db.QueryContext(ctx, `SELECT "table","from" FROM pragma_foreign_key_list(?)`, tb)
		if err != nil {
			t.Fatal(err)
		}
		type f struct{ parent, col string }
		var fks []f
		for fk.Next() {
			var x f
			if err := fk.Scan(&x.parent, &x.col); err != nil {
				t.Fatal(err)
			}
			fks = append(fks, x)
		}
		fk.Close()
		for _, x := range fks {
			var n int
			err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM pragma_index_list(?) il JOIN pragma_index_info(il.name) ii
				WHERE ii.seqno = 0 AND ii.name = ?`, tb, x.col).Scan(&n)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				t.Errorf("%s.%s refers to %s but leads no index", tb, x.col, x.parent)
			}
		}
	}
}
