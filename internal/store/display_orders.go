package store

import "context"

// SeriesDisplayOrder is the selected order and its browse/playback traversal.
type SeriesDisplayOrder struct {
	Source  string
	Entries []OrderingEntry
}

// SeriesDisplayOrders resolves the same choices as DisplayOrdering and DisplayEntries for a
// bounded group of series. Read entries and applicable placements in the same query.
func (s *Store) SeriesDisplayOrders(ctx context.Context, seriesIDs []string) (map[string]SeriesDisplayOrder, error) {
	orders := make(map[string]SeriesDisplayOrder, len(seriesIDs))
	for ids := range chunks(seriesIDs, 100) {
		args := make([]any, len(ids))
		for i, id := range ids {
			args[i] = id
		}
		rows, err := s.db.QueryContext(ctx, `
			SELECT sr.id, o.source, e.episode_id, e.season_number, e.episode_number, e.locked,
				COALESCE(p.before_episode_id, '')
			FROM items sr JOIN libraries l ON l.id = sr.library_id
			LEFT JOIN series_display d ON d.series_id = sr.id AND d.user_id = ''
			JOIN orderings o ON o.id = COALESCE(d.ordering_id, (
				SELECT id FROM orderings WHERE series_id = sr.id
				ORDER BY `+displayOrderingPriority("COALESCE(NULLIF(sr.source_ordering, ''), l.source_ordering)")+` LIMIT 1))
			JOIN ordering_entries e ON e.ordering_id = o.id
			LEFT JOIN series_specials sp ON sp.series_id = o.series_id AND sp.user_id = ''
			LEFT JOIN episode_specials p ON p.episode_id = e.episode_id AND sp.inline = 1 AND o.source <> 'custom'
				AND EXISTS (SELECT 1 FROM items ep WHERE ep.id = p.episode_id AND ep.parent_id = o.series_id)
			WHERE sr.id IN (`+placeholders(len(ids))+`)
			ORDER BY sr.id, e.season_number, e.episode_number`, args...)
		if err != nil {
			return nil, err
		}
		placements := map[string]map[string]string{}
		for rows.Next() {
			var id, source, before string
			var entry OrderingEntry
			if err := rows.Scan(&id, &source, &entry.EpisodeID, &entry.Season, &entry.Episode, &entry.Locked, &before); err != nil {
				rows.Close()
				return nil, err
			}
			order := orders[id]
			order.Source = source
			order.Entries = append(order.Entries, entry)
			orders[id] = order
			if before != "" {
				if placements[id] == nil {
					placements[id] = map[string]string{}
				}
				placements[id][entry.EpisodeID] = before
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			if order, ok := orders[id]; ok {
				order.Entries = placeSpecials(order.Entries, placements[id])
				orders[id] = order
			}
		}
	}
	return orders, nil
}
