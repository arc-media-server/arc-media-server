package store

import "context"

// PendingFormats is deliberately capped. It never resets probed_at or changes chapter,
// matching, watch-state or presence data. Missing/unreadable files are excluded.
func (s *Store) PendingFormats(ctx context.Context, libraryID, seenSince int64) ([]MediaFile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, path, size, mtime FROM media_files m
 WHERE library_id=? AND format_probe_version=0 AND probed_at IS NOT NULL
 AND probe_error='' AND missing_since IS NULL AND last_seen_at >= ? AND EXISTS
 (SELECT 1 FROM media_streams st WHERE st.file_id=m.id AND st.kind='video'
 AND (st.codec IN ('hevc','h265','av1') OR st.color_transfer IN ('smpte2084','arib-std-b67')))
 ORDER BY path LIMIT 32`, libraryID, seenSince)
	if err != nil {
		return nil, err
	}
	var files []MediaFile
	for rows.Next() {
		var f MediaFile
		f.LibraryID = libraryID
		if err := rows.Scan(&f.ID, &f.Path, &f.Size, &f.ModTime); err != nil {
			rows.Close()
			return nil, err
		}
		files = append(files, f)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range files {
		files[i].Streams, err = s.fileStreams(ctx, files[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

// SaveFormats protects against a concurrent regular probe/change: only the old pending
// revision can be enriched. A failed optional sample is recorded as attempted too.
func (s *Store) SaveFormats(ctx context.Context, f MediaFile) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, `UPDATE media_files SET format_probe_version=1 WHERE id=? AND size=? AND mtime=? AND format_probe_version=0`, f.ID, f.Size, f.ModTime)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	for _, st := range f.Streams {
		_, err = tx.ExecContext(ctx, `UPDATE media_streams SET profile=?,channel_layout=?,dolby_vision=?,dolby_vision_profile=?,hdr10_plus=?,hdr_probe_status=? WHERE file_id=? AND idx=? AND kind=? AND codec=?`, st.Profile, st.ChannelLayout, boolInt(st.DolbyVision), st.DolbyVisionProfile, boolInt(st.HDR10Plus), st.HDRProbeStatus, f.ID, st.Index, st.Kind, st.Codec)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
