package store

import (
	"context"
	"path/filepath"
)

// ReconcileMovieFiles marks missing files and records replacements in one transaction.
// The caller has checked that the scanned folders and their storage are available.
// As in MarkMissing, nil folders also reconciles files under former library roots.
func (s *Store) ReconcileMovieFiles(ctx context.Context, libraryID, scanStart, now int64, folders []string) (missing, replaced int64, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	// Look for a single freshly discovered file and a single previously available file
	// for the same item. A previously discovered alternate that coexisted with the old
	// file, a returning file or an unchanged rename gives no evidence of replacement.
	rows, err := tx.QueryContext(ctx, `
		SELECT i.id, old.path, fresh.path
		FROM media_files old
		JOIN item_files former ON former.file_id=old.id AND former.part=0
		JOIN items i ON i.id=former.item_id AND i.library_id=old.library_id AND i.kind='movie'
		JOIN item_files replacement ON replacement.item_id=i.id AND replacement.part=0 AND replacement.file_id<>old.id
		JOIN media_files fresh ON fresh.id=replacement.file_id AND fresh.library_id=old.library_id
		WHERE old.library_id=? AND old.missing_since IS NULL AND old.last_seen_at<?
			AND fresh.missing_since IS NULL AND fresh.last_seen_at=? AND fresh.first_seen_at>old.last_seen_at
			AND old.size>0 AND fresh.size>0 AND fresh.size<>old.size
			AND NOT EXISTS (
				SELECT 1 FROM item_files other JOIN media_files m ON m.id=other.file_id
				WHERE other.item_id=i.id AND m.id<>old.id AND m.id<>fresh.id AND m.missing_since IS NULL
			)
		ORDER BY fresh.first_seen_at, fresh.id, i.id`, libraryID, scanStart, scanStart)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	var replacements []string
	for rows.Next() {
		var id, oldPath, newPath string
		if err := rows.Scan(&id, &oldPath, &newPath); err != nil {
			return 0, 0, err
		}
		// A new file seen in this scan proves the shared folder was visited.
		// Keep moves between folders or roots out of the upgrade heuristic.
		if filepath.Dir(oldPath) == filepath.Dir(newPath) {
			replacements = append(replacements, id)
		}
	}
	if err := rows.Close(); err != nil {
		return 0, 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}

	missing, err = markMissing(ctx, tx, libraryID, scanStart, now, folders)
	if err != nil {
		return 0, 0, err
	}
	for _, id := range replacements {
		recency, err := nextArrival(ctx, tx)
		if err != nil {
			return 0, 0, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE items SET movie_replaced_at=?,movie_replacement_sequence=? WHERE id=?`,
			now, recency.sequence, id); err != nil {
			return 0, 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return missing, int64(len(replacements)), nil
}
