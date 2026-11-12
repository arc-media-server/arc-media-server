package store

import (
	"context"
	"encoding/json"
	"time"
)

// RenameCandidates is bounded and indexed; fingerprints are candidates, not proof of identity.
func (s *Store) RenameCandidates(ctx context.Context, lib, size, mtime int64) ([]MediaFile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+fileCols+` FROM media_files WHERE library_id=? AND size=? AND mtime=? LIMIT 33`, lib, size, mtime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var files []MediaFile
	for rows.Next() {
		var f MediaFile
		if err := scanFile(rows, &f); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// RelocateFile changes only the catalog path, retaining streams, markers and other file references.
func (s *Store) RelocateFile(ctx context.Context, f MediaFile, path string) (bool, error) {
	r, err := s.db.ExecContext(ctx, `UPDATE media_files SET path=? WHERE id=? AND path=? AND size=? AND mtime=?`, path, f.ID, f.Path, f.Size, f.ModTime)
	if err != nil {
		return false, err
	}
	n, err := r.RowsAffected()
	return n == 1, err
}
func (s *Store) OwnersForFiles(ctx context.Context, libraryID int64, kind string, ids []int64) ([]Item, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	input, err := json.Marshal(ids)
	if err != nil {
		return nil, err
	}
	return s.queryItems(ctx, `SELECT DISTINCT `+prefixCols("owner.", itemCols)+` FROM item_files f
 JOIN items linked ON linked.id=f.item_id
 JOIN items owner ON owner.id=CASE WHEN linked.kind='episode' THEN linked.parent_id ELSE linked.id END
 WHERE f.file_id IN (SELECT value FROM json_each(?)) AND owner.library_id=? AND owner.kind=? LIMIT 2`, string(input), libraryID, kind)
}

// RekeyItemWithAlias preserves old scanner keys for other versions/folders still using that name.
func (s *Store) RekeyItemWithAlias(ctx context.Context, id, key string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO item_aliases(library_id,match_key,item_id) SELECT library_id,match_key,id FROM items WHERE id=?`, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE items SET match_key=?,updated_at=? WHERE id=?`, key, time.Now().Unix(), id); err != nil {
		return err
	}
	return tx.Commit()
}
