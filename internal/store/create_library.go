package store

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"
)

var ErrLibraryPathConflict = errors.New("a folder is already used by another library")

// CreateConfiguredLibrary publishes the validated core, paths and scalar options together.
// API filesystem checks happen beforehand; this transaction rechecks current DB ownership.
func (s *Store) CreateConfiguredLibrary(ctx context.Context, l Library) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var exists bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM libraries WHERE name=? COLLATE NOCASE)`, l.Name).Scan(&exists); err != nil {
		return 0, err
	}
	if exists {
		return 0, ErrDuplicateName
	}
	rows, err := tx.QueryContext(ctx, `SELECT path FROM library_paths`)
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		var old string
		if err = rows.Scan(&old); err != nil {
			rows.Close()
			return 0, err
		}
		old = strings.ToLower(filepath.Clean(old))
		for _, path := range l.Paths {
			path = strings.ToLower(filepath.Clean(path))
			if underAny(path, []string{old}) || underAny(old, []string{path}) {
				rows.Close()
				return 0, ErrLibraryPathConflict
			}
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	kind := l.Kind
	videos := kind == "videos"
	if videos {
		kind = "movies"
	}
	var id int64
	err = tx.QueryRowContext(ctx, `INSERT INTO libraries(name,kind,source_ordering,videos,created_at,trickplay,collection_min,metadata_language,rating_region,skip_intros,skip_credits,collection_grid,details_provider,season_label_mode)
 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,COALESCE(NULLIF(?, ''), 'number')) RETURNING id`, l.Name, kind, l.SourceOrdering, videos, time.Now().Unix(), l.Trickplay, l.CollectionMin, l.MetadataLanguage, l.RatingRegion, l.SkipIntros, l.SkipCredits, l.CollectionGrid, l.DetailsProvider, l.SeasonLabelMode).Scan(&id)
	if err != nil {
		return 0, err
	}
	for _, path := range l.Paths {
		if _, err = tx.ExecContext(ctx, `INSERT INTO library_paths(library_id,path) VALUES (?,?)`, id, path); err != nil {
			return 0, err
		}
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}
