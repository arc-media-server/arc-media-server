package store

import "context"

// RefreshPage is an ID cursor, not an offset or a maximum operation size.
func (s *Store) RefreshPage(ctx context.Context, libraryID int64, after string) ([]Item, error) {
	return s.queryItems(ctx, `SELECT `+itemCols+` FROM items WHERE library_id=?
    AND kind IN ('movie','series') AND id>? ORDER BY id LIMIT 200`, libraryID, after)
}

// AnalysisPage visits known present files without walking folders or reconciling absence.
func (s *Store) AnalysisPage(ctx context.Context, libraryID, after int64) ([]MediaFile, error) {
	return s.queryFiles(ctx, `SELECT `+fileCols+` FROM media_files
    WHERE library_id=? AND id>? AND missing_since IS NULL ORDER BY id LIMIT 200`, libraryID, after)
}

// ItemAnalysisFiles lists the present files of one movie or episode, or of every episode of a
// show, for analysing a single title.
func (s *Store) ItemAnalysisFiles(ctx context.Context, itemID string) ([]MediaFile, error) {
	return s.queryFiles(ctx, `SELECT `+fileCols+` FROM media_files WHERE missing_since IS NULL AND id IN (
    SELECT file_id FROM item_files WHERE item_id=? OR item_id IN (SELECT id FROM items WHERE parent_id=?))
    ORDER BY path`, itemID, itemID)
}

func (s *Store) queryFiles(ctx context.Context, q string, args ...any) ([]MediaFile, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
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
