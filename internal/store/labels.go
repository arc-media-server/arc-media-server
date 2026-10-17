package store

import (
	"context"
	"strings"
)

// ItemLabels returns a movie or show's labels.
func (s *Store) ItemLabels(ctx context.Context, itemID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT label FROM item_labels WHERE item_id = ? ORDER BY rowid`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// AllLabels lists every label in use, for suggestions.
func (s *Store) AllLabels(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT label FROM item_labels GROUP BY label ORDER BY label COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// labelsHiddenSQL is a condition that an item (alias; episodes by their show) has none of n
// labels.
func labelsHiddenSQL(alias string, n int) string {
	a := alias + "."
	return `NOT EXISTS (SELECT 1 FROM item_labels l WHERE l.item_id = (CASE WHEN ` + a + `kind = 'episode' THEN ` +
		a + `parent_id ELSE ` + a + `id END) AND l.label IN (` + strings.TrimSuffix(strings.Repeat("?,", n), ",") + `))`
}
