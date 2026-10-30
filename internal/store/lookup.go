package store

import (
	"context"
	"errors"
)

// LookupItems returns every accessible matching identity, not an arbitrary first library/edition.
func (s *Store) LookupItems(ctx context.Context, provider, id, kind string, library int64, limit, offset int, acc Access) ([]Item, int, error) {
	columns := map[string]string{"tmdb": "tmdb_id", "tvdb": "tvdb_id", "imdb": "imdb_id COLLATE NOCASE"}
	column, ok := columns[provider]
	if !ok {
		return nil, 0, errors.New("invalid provider")
	}
	query := " FROM items WHERE " + column + "=?"
	args := []any{id}
	if kind != "" {
		query += " AND kind=?"
		args = append(args, kind)
	}
	if library > 0 {
		query += " AND library_id=?"
		args = append(args, library)
	}
	query, args = acc.Where("items", query, args)
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*)"+query, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	items, err := s.queryItems(ctx, "SELECT "+itemCols+query+" ORDER BY id LIMIT ? OFFSET ?", args...)
	return items, total, err
}

// VisibleItemsByID supports bounded automation reads, including identities temporarily offline.
func (s *Store) VisibleItemsByID(ctx context.Context, ids []string, acc Access) (map[string]Item, error) {
	out := map[string]Item{}
	for chunk := range chunks(ids, 100) {
		args := make([]any, len(chunk))
		for i, id := range chunk {
			args[i] = id
		}
		q, args := acc.Where("items", "SELECT "+itemCols+" FROM items WHERE id IN ("+placeholders(len(chunk))+")", args)
		items, err := s.queryItems(ctx, q, args...)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			out[it.ID] = it
		}
	}
	return out, nil
}
