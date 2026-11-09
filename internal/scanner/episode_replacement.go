package scanner

import (
	"ams/internal/store"
	"context"
)

// priorSourceEpisodes records stable file positions before refreshing provider entries. Ambiguous
// positions are deliberately unusable; custom/display orderings are not filename identities.
func (s *Scanner) priorSourceEpisodes(ctx context.Context, seriesID, source string) (map[string]string, error) {
	orders, err := s.store.ListOrderings(ctx, seriesID)
	if err != nil {
		return nil, err
	}
	positions := map[string]string{}
	for _, order := range orders {
		if order.Source != source || order.SourceRef != "" {
			continue
		}
		entries, err := s.store.OrderingEntries(ctx, order.ID)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			key := fileEpisodeKey(seriesID, source == "tvdb_absolute", entry.Season, entry.Episode)
			if _, exists := positions[key]; exists {
				positions[key] = ""
			} else {
				positions[key] = entry.EpisodeID
			}
		}
	}
	return positions, nil
}
func (s *Scanner) adoptReplacedEpisode(ctx context.Context, libraryID int64, newKey, oldID string, previous map[string]store.EpisodeIdentity, currentKeys map[string]bool) error {
	prior, ok := previous[oldID]
	if !ok || !prior.OwnsFiles || currentKeys[prior.MatchKey] {
		return nil
	}
	// Existing new identities and known cross-provider mappings win over positional inference.
	target, err := s.store.GetItemByKey(ctx, libraryID, newKey)
	if err != nil || target != nil {
		return err
	}
	// Another position in this refresh may already have adopted this old key.
	old, err := s.store.GetItemByKey(ctx, libraryID, prior.MatchKey)
	if err != nil || old == nil || old.ID != oldID {
		return err
	}
	return s.store.RekeyItem(ctx, oldID, newKey)
}
