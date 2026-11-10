package store

import "context"

// PlaylistPosterItems returns the first four distinct image identities, without
// loading every full entry. Episodes use their parent's poster even when empty,
// matching the API's existing placeholder behavior; movies without art are skipped.
func (s *Store) PlaylistPosterItems(ctx context.Context, id string, acc Access) ([]Item, error) {
	q := "SELECT items.id AS item_id, e.position AS entry_position, e.id AS entry_id, ROW_NUMBER() OVER(PARTITION BY CASE WHEN items.kind='episode' THEN items.parent_id ELSE items.id END ORDER BY e.position,e.id) AS poster_rank FROM playlist_entries e JOIN items ON items.id=e.item_id WHERE e.playlist_id=? AND " + playableSQL + " AND (items.kind='episode' OR items.poster_path<>'' OR items.local_poster<>'')"
	q, args := acc.Where("items", q, []any{id})
	return s.queryItems(ctx, "SELECT "+prefixCols("items.", itemCols)+" FROM ("+q+") ranked JOIN items ON items.id=ranked.item_id WHERE poster_rank=1 ORDER BY entry_position,entry_id LIMIT 4", args...)
}

// PlaylistMovieSummaries projects only fields used by a smart-playlist summary.
// Its ordering/filter/window are the same as ListItems; full playback/edit pages
// continue to use full items. The summary's count is the bounded window length.
func (s *Store) PlaylistMovieSummaries(ctx context.Context, q ItemQuery) ([]Item, error) {
	cond, args := itemFilter(q)
	order, orderArgs := itemOrdering(q)
	limit := q.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	all := append(append(append([]any{}, args...), orderArgs...), limit, max(0, q.Offset))
	rows, err := s.db.QueryContext(ctx, "SELECT id,kind,COALESCE(parent_id, ''),poster_path,local_poster,runtime_min,locked_fields FROM items WHERE "+cond+" ORDER BY "+order+" LIMIT ? OFFSET ?", all...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		var it Item
		var locked string
		if err := rows.Scan(&it.ID, &it.Kind, &it.ParentID, &it.PosterPath, &it.LocalPoster, &it.RuntimeMin, &locked); err != nil {
			return nil, err
		}
		it.LockedFields = splitLocked(locked)
		out = append(out, it)
	}
	return out, rows.Err()
}
