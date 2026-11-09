package store

import "context"

// EpisodeIdentity snapshots keys and file ownership before a provider response replaces orders.
type EpisodeIdentity struct {
	MatchKey  string
	OwnsFiles bool
}

func (s *Store) EpisodeIdentities(ctx context.Context, seriesID string) (map[string]EpisodeIdentity, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT i.id,i.match_key,EXISTS(SELECT 1 FROM item_files f WHERE f.item_id=i.id)
 FROM items i WHERE i.parent_id=? AND i.kind='episode'`, seriesID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]EpisodeIdentity{}
	for rows.Next() {
		var id string
		var identity EpisodeIdentity
		if err := rows.Scan(&id, &identity.MatchKey, &identity.OwnsFiles); err != nil {
			return nil, err
		}
		result[id] = identity
	}
	return result, rows.Err()
}
