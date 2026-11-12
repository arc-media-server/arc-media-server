package scanner

import (
	"ams/internal/metadata"
	"context"
	"fmt"
)

// Validate before replacing any saved metadata/orderings. A successful HTTP response is not
// proof that a provider has returned a complete episode list.
func (s *Scanner) validateProviderEpisodes(ctx context.Context, seriesID, preferred string, incoming []metadata.Ordering) error {
	prior, err := s.store.ListOrderings(ctx, seriesID)
	if err != nil {
		return err
	}
	oldSources := map[string]bool{}
	for _, order := range prior {
		if order.SourceRef == "" {
			oldSources[order.Source] = true
		}
	}
	oldSource := pickSource(preferred, func(source string) bool { return oldSources[source] })
	if oldSource == "" {
		return nil
	}
	oldID := int64(0)
	for _, order := range prior {
		if order.Source == oldSource && order.SourceRef == "" {
			oldID = order.ID
			break
		}
	}
	entries, err := s.store.OrderingEntries(ctx, oldID)
	if err != nil {
		return err
	}
	oldIDs := map[string]bool{}
	for _, entry := range entries {
		oldIDs[entry.EpisodeID] = true
	}
	if len(oldIDs) == 0 {
		return nil
	}
	newSources := map[string]map[int64]bool{}
	for _, order := range incoming {
		if order.SourceRef != "" {
			continue
		}
		ids := map[int64]bool{}
		for _, entry := range order.Entries {
			if entry.Episode != nil {
				ids[entry.Episode.ID] = true
			}
		}
		newSources[order.Source] = ids
	}
	if oldSources[preferred] && newSources[preferred] == nil {
		return fmt.Errorf("provider episode response omitted saved source order %s; keeping previous episodes", preferred)
	}
	newSource := pickSource(preferred, func(source string) bool { return newSources[source] != nil })
	next := len(newSources[newSource])
	// File-only counts are naming observations, not a prior provider catalog. A first match may
	// legitimately cover fewer files (specials/unrecognised numbering), but not an empty catalog.
	if next == 0 || oldSource != "file" && next*2 < len(oldIDs) {
		return fmt.Errorf("provider episode response for %s dropped from %d to %d; keeping previous episodes", newSource, len(oldIDs), next)
	}
	return nil
}
