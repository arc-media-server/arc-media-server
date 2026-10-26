package store

import "context"

// TrashTitleSample supplies a bounded Activity summary, using the exact deletion predicate.
func (s *Store) TrashTitleSample(ctx context.Context, libraryID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT title FROM items WHERE id IN (`+goneItemsSQL+`) ORDER BY title LIMIT 5`, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	titles := []string{}
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			return nil, err
		}
		titles = append(titles, title)
	}
	return titles, rows.Err()
}

// PresentLibraryFiles supplies lightweight filesystem anchors for conservative automatic
// cleanup. Unlinked files count too; metadata matching is independent of root availability.
func (s *Store) PresentLibraryFiles(ctx context.Context, libraryID int64) ([]MediaFile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT path, size FROM media_files WHERE library_id = ? AND missing_since IS NULL`, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	files := []MediaFile{}
	for rows.Next() {
		var f MediaFile
		if err := rows.Scan(&f.Path, &f.Size); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}
