package store

import "context"

// RefreshPage is an ID cursor, not an offset or a maximum operation size.
func (s *Store) RefreshPage(ctx context.Context, libraryID int64, after string) ([]Item, error) {
	return s.queryItems(ctx, `SELECT `+itemCols+` FROM items WHERE library_id=?
    AND kind IN ('movie','series') AND id>? ORDER BY id LIMIT 200`, libraryID, after)
}

// AnalysisPage visits known present files without walking folders or reconciling absence.
func (s *Store) AnalysisPage(ctx context.Context, libraryID, after int64) ([]MediaFile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+fileCols+` FROM media_files
    WHERE library_id=? AND id>? AND missing_since IS NULL ORDER BY id LIMIT 200`, libraryID, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MediaFile{}
	for rows.Next() {
		var file MediaFile
		if err := scanFile(rows, &file); err != nil {
			return nil, err
		}
		out = append(out, file)
	}
	return out, rows.Err()
}
