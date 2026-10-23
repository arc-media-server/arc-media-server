package store

import "context"

// SaveFormats protects against a concurrent regular probe/change: only the old pending
// revision can be enriched. A failed optional sample is recorded as attempted too.
func (s *Store) SaveFormats(ctx context.Context, f MediaFile) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, `UPDATE media_files SET format_probe_version=1 WHERE id=? AND path=? AND size=? AND mtime=? AND format_probe_version=0`, f.ID, f.Path, f.Size, f.ModTime)
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
