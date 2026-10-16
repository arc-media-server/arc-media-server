package store

import (
	"context"
	"database/sql"
	"errors"
)

const fileCols = `id, library_id, path, size, mtime, container, duration_ms, bitrate, probe_error,
	probed_at IS NOT NULL, missing_since IS NOT NULL`

func scanFile(row interface{ Scan(...any) error }, f *MediaFile) error {
	return row.Scan(&f.ID, &f.LibraryID, &f.Path, &f.Size, &f.ModTime, &f.Container, &f.DurationMS,
		&f.Bitrate, &f.ProbeError, &f.Probed, &f.Missing)
}

// GetFileByPath returns nil when the path has never been scanned.
func (s *Store) GetFileByPath(ctx context.Context, path string) (*MediaFile, error) {
	var f MediaFile
	err := scanFile(s.db.QueryRowContext(ctx, `SELECT `+fileCols+` FROM media_files WHERE path = ?`, path), &f)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &f, err
}

// SaveFile inserts or updates a file and replaces its streams. seenAt marks it present in the current scan.
func (s *Store) SaveFile(ctx context.Context, f *MediaFile, probedAt, seenAt int64) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var probed any
	if probedAt > 0 {
		probed = probedAt
	}
	var id int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO media_files (library_id, path, size, mtime, container, duration_ms, bitrate,
			probed_at, probe_error, last_seen_at, missing_since)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)
		ON CONFLICT (path) DO UPDATE SET
			library_id = excluded.library_id, size = excluded.size, mtime = excluded.mtime,
			container = excluded.container, duration_ms = excluded.duration_ms, bitrate = excluded.bitrate,
			probed_at = excluded.probed_at, probe_error = excluded.probe_error,
			last_seen_at = excluded.last_seen_at, missing_since = NULL
		RETURNING id`,
		f.LibraryID, f.Path, f.Size, f.ModTime, f.Container, f.DurationMS, f.Bitrate,
		probed, f.ProbeError, seenAt).Scan(&id)
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM media_streams WHERE file_id = ?`, id); err != nil {
		return 0, err
	}
	for _, st := range f.Streams {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO media_streams (file_id, idx, kind, codec, profile, language, title, width, height,
				channels, is_default, is_forced)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, st.Index, st.Kind, st.Codec, st.Profile, st.Language, st.Title, st.Width, st.Height,
			st.Channels, boolInt(st.IsDefault), boolInt(st.IsForced)); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// TouchFile marks an unchanged file as seen in the current scan.
func (s *Store) TouchFile(ctx context.Context, id, seenAt int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE media_files SET last_seen_at = ?, missing_since = NULL WHERE id = ?`, seenAt, id)
	return err
}

// MarkMissing flags files in a library that were not seen since scanStart.
func (s *Store) MarkMissing(ctx context.Context, libraryID, scanStart, now int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE media_files SET missing_since = ?
		WHERE library_id = ? AND last_seen_at < ? AND missing_since IS NULL`, now, libraryID, scanStart)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// SetFileItems sets exactly which items a file belongs to, detaching it from any others (e.g.
// after a rename changed what the file parses as). A multi-episode file belongs to several items.
func (s *Store) SetFileItems(ctx context.Context, fileID int64, itemIDs []string, part int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM item_files WHERE file_id = ?`, fileID); err != nil {
		return err
	}
	for _, id := range itemIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO item_files (item_id, file_id, part) VALUES (?, ?, ?)`, id, fileID, part); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ItemFiles returns an item's files (including missing ones) with their streams, ordered by part.
func (s *Store) ItemFiles(ctx context.Context, itemID string) ([]MediaFile, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+prefixCols("m.", fileCols)+`, f.part
		FROM item_files f JOIN media_files m ON m.id = f.file_id
		WHERE f.item_id = ? ORDER BY f.part, m.path`, itemID)
	if err != nil {
		return nil, err
	}
	var files []MediaFile
	for rows.Next() {
		var f MediaFile
		if err := rows.Scan(&f.ID, &f.LibraryID, &f.Path, &f.Size, &f.ModTime, &f.Container, &f.DurationMS,
			&f.Bitrate, &f.ProbeError, &f.Probed, &f.Missing, &f.Part); err != nil {
			rows.Close()
			return nil, err
		}
		files = append(files, f)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range files {
		if files[i].Streams, err = s.fileStreams(ctx, files[i].ID); err != nil {
			return nil, err
		}
	}
	return files, nil
}

// SeriesFilePaths returns up to limit file paths of a series' episodes, for explaining how the
// series was read from its folder.
func (s *Store) SeriesFilePaths(ctx context.Context, seriesID string, limit int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT m.path FROM items i
		JOIN item_files f ON f.item_id = i.id JOIN media_files m ON m.id = f.file_id
		WHERE i.parent_id = ? ORDER BY m.missing_since IS NOT NULL, m.path LIMIT ?`, seriesID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) fileStreams(ctx context.Context, fileID int64) ([]Stream, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT idx, kind, codec, profile, language, title, width, height, channels, is_default, is_forced
		FROM media_streams WHERE file_id = ? ORDER BY idx`, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	streams := []Stream{}
	for rows.Next() {
		var st Stream
		if err := rows.Scan(&st.Index, &st.Kind, &st.Codec, &st.Profile, &st.Language, &st.Title,
			&st.Width, &st.Height, &st.Channels, &st.IsDefault, &st.IsForced); err != nil {
			return nil, err
		}
		streams = append(streams, st)
	}
	return streams, rows.Err()
}
