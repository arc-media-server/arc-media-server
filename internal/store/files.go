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
	if probedAt > 0 {
		if _, err := tx.ExecContext(ctx, `DELETE FROM media_chapters WHERE file_id = ?`, id); err != nil {
			return 0, err
		}
		for i, c := range f.Chapters {
			if _, err := tx.ExecContext(ctx, `INSERT INTO media_chapters (file_id, idx, start_ms, end_ms, title) VALUES (?, ?, ?, ?, ?)`,
				id, i, c.StartMS, c.EndMS, c.Title); err != nil {
				return 0, err
			}
		}
	}
	for _, st := range f.Streams {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO media_streams (file_id, idx, kind, codec, profile, language, title, width, height,
				channels, is_default, is_forced, pix_fmt, bit_depth, color_transfer, color_primaries, frame_rate, bitrate)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, st.Index, st.Kind, st.Codec, st.Profile, st.Language, st.Title, st.Width, st.Height,
			st.Channels, boolInt(st.IsDefault), boolInt(st.IsForced), st.PixFmt, st.BitDepth, st.ColorTransfer,
			st.ColorPrimaries, st.FrameRate, st.Bitrate); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// FileChapters returns a file's chapter marks in order.
func (s *Store) FileChapters(ctx context.Context, fileID int64) ([]Chapter, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT start_ms, end_ms, title FROM media_chapters WHERE file_id = ? ORDER BY idx`, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Chapter{}
	for rows.Next() {
		var c Chapter
		if err := rows.Scan(&c.StartMS, &c.EndMS, &c.Title); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
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
		f.Edition = EditionOf(f.Path)
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
		SELECT idx, kind, codec, profile, language, title, width, height, channels, is_default, is_forced,
			pix_fmt, bit_depth, color_transfer, color_primaries, frame_rate, bitrate
		FROM media_streams WHERE file_id = ? ORDER BY idx`, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	streams := []Stream{}
	for rows.Next() {
		var st Stream
		if err := rows.Scan(&st.Index, &st.Kind, &st.Codec, &st.Profile, &st.Language, &st.Title,
			&st.Width, &st.Height, &st.Channels, &st.IsDefault, &st.IsForced, &st.PixFmt, &st.BitDepth,
			&st.ColorTransfer, &st.ColorPrimaries, &st.FrameRate, &st.Bitrate); err != nil {
			return nil, err
		}
		streams = append(streams, st)
	}
	return streams, rows.Err()
}

// VideoFile is a present, readable movie or episode file and its picture size.
type VideoFile struct {
	ID            int64
	Path          string
	DurationMS    int64
	Width, Height int
	// The colour transfer (smpte2084 or arib-std-b67 for HDR).
	ColorTransfer string
}

// VideoFiles lists present, readable movie and episode files in libraries with timeline
// thumbnails turned on, newest titles first.
func (s *Store) VideoFiles(ctx context.Context) ([]VideoFile, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.path, m.duration_ms,
			COALESCE((SELECT width FROM media_streams st WHERE st.file_id = m.id AND st.kind = 'video' ORDER BY idx LIMIT 1), 0),
			COALESCE((SELECT height FROM media_streams st WHERE st.file_id = m.id AND st.kind = 'video' ORDER BY idx LIMIT 1), 0),
			COALESCE((SELECT color_transfer FROM media_streams st WHERE st.file_id = m.id AND st.kind = 'video' ORDER BY idx LIMIT 1), '')
		FROM media_files m
		JOIN item_files f ON f.file_id = m.id JOIN items i ON i.id = f.item_id
		JOIN libraries l ON l.id = i.library_id AND l.trickplay = 1
		WHERE m.missing_since IS NULL AND m.probed_at IS NOT NULL AND m.duration_ms > 0
			AND i.kind IN ('movie', 'episode')
		GROUP BY m.id
		ORDER BY MAX(i.added_at) DESC, m.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VideoFile
	for rows.Next() {
		var v VideoFile
		if err := rows.Scan(&v.ID, &v.Path, &v.DurationMS, &v.Width, &v.Height, &v.ColorTransfer); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// LibraryItemFiles returns the present files of a library's movies and episodes, by item id.
func (s *Store) LibraryItemFiles(ctx context.Context, libraryID int64) (map[string][]MediaFile, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT f.item_id, `+prefixCols("m.", fileCols)+`, f.part
		FROM item_files f JOIN media_files m ON m.id = f.file_id JOIN items i ON i.id = f.item_id
		WHERE i.library_id = ? AND i.kind IN ('movie', 'episode') AND m.missing_since IS NULL
		ORDER BY f.item_id, f.part, m.path`, libraryID)
	if err != nil {
		return nil, err
	}
	out := map[string][]MediaFile{}
	var all []*MediaFile
	for rows.Next() {
		var itemID string
		var f MediaFile
		if err := rows.Scan(&itemID, &f.ID, &f.LibraryID, &f.Path, &f.Size, &f.ModTime, &f.Container, &f.DurationMS,
			&f.Bitrate, &f.ProbeError, &f.Probed, &f.Missing, &f.Part); err != nil {
			rows.Close()
			return nil, err
		}
		f.Edition = EditionOf(f.Path)
		out[itemID] = append(out[itemID], f)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for id := range out {
		for i := range out[id] {
			all = append(all, &out[id][i])
		}
	}
	for _, f := range all {
		if f.Streams, err = s.fileStreams(ctx, f.ID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// UnprobedFile is a present file not yet read with ffprobe.
type UnprobedFile struct {
	ID       int64
	Path     string
	Size     int64
	ModTime  int64
	LastSeen int64
}

// UnprobedFiles lists a library's present files that haven't been read yet, in path order.
func (s *Store) UnprobedFiles(ctx context.Context, libraryID int64) ([]UnprobedFile, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, path, size, mtime, last_seen_at FROM media_files
		WHERE library_id = ? AND probed_at IS NULL AND missing_since IS NULL ORDER BY path`, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UnprobedFile
	for rows.Next() {
		var f UnprobedFile
		if err := rows.Scan(&f.ID, &f.Path, &f.Size, &f.ModTime, &f.LastSeen); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
