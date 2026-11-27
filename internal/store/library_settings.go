package store

import (
	"context"
	"errors"
)

var ErrLibraryNotFound = errors.New("library not found")

// SaveLibrarySettings commits all scalar controls, folder changes and metadata invalidation
// together. The API holds the scanner's existing per-library work lease over its read/apply.
func (s *Store) SaveLibrarySettings(ctx context.Context, before, after Library, core bool) error {
	var plan *libraryRelocation
	var err error
	if core {
		plan, err = s.prepareLibraryRelocation(ctx, after.ID, after.Paths)
		if err != nil {
			return err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if core {
		if err := updateLibraryTx(ctx, tx, after.ID, after.Name, after.SourceOrdering, after.Paths, after.DetailsProvider, plan); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE libraries SET trickplay=?, collection_min=?,metadata_language=?,rating_region=?,skip_intros=?,skip_credits=?,collection_grid=?,details_provider=?,season_label_mode=? WHERE id=?`, boolInt(after.Trickplay), after.CollectionMin, after.MetadataLanguage, after.RatingRegion, boolInt(after.SkipIntros), boolInt(after.SkipCredits), after.CollectionGrid, after.DetailsProvider, after.SeasonLabelMode, after.ID)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return ErrLibraryNotFound
	}
	if before.MetadataLanguage != after.MetadataLanguage {
		_, err = tx.ExecContext(ctx, `UPDATE items SET synced_at=0,metadata_state='pending' WHERE library_id=? AND kind IN ('movie','series') AND metadata_state<>'locked'`, after.ID)
		if err != nil {
			return err
		}
	}
	if before.RatingRegion != after.RatingRegion {
		_, err = tx.ExecContext(ctx, `UPDATE items SET synced_at=0 WHERE library_id=? AND kind IN ('movie','series')`, after.ID)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
