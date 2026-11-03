package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"time"
)

// Optimized versions: easy-to-play copies of files (H.264 and AAC in MP4, at most 1080p or 720p),
// made ahead of time in Arc's data folder so weak or remote players needn't convert live.

// Optimized is one optimized copy of a file.
type Optimized struct {
	FileID     int64  `json:"file_id"`
	Quality    int    `json:"quality"` // the most lines tall: 1080 or 720
	Path       string `json:"-"`
	Size       int64  `json:"size"`
	DurationMS int64  `json:"duration_ms,omitempty"`
	Bitrate    int64  `json:"bitrate,omitempty"`
	// queued, converting, done or failed.
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	// What it's of, for the list in Settings.
	ItemID      string `json:"item_id"`
	Title       string `json:"title"`
	SeriesTitle string `json:"series_title,omitempty"`
	CreatedAt   int64  `json:"created_at"`
	// Source and attempt identity are internal, never a promise about playback capabilities.
	SourcePath    string `json:"-"`
	SourceSize    int64  `json:"-"`
	SourceMTime   int64  `json:"-"`
	SourceMTimeNS int64  `json:"-"`
	JobID         string `json:"-"`
	// The mapped source audio, not an output-container stream index. -2 means unknown,
	// -1 means a silent copy; otherwise the output is AAC stereo from this exact track.
	AudioStream   int    `json:"-"`
	AudioLanguage string `json:"-"`
	AudioTitle    string `json:"-"`
}

const optimizedCols = `o.file_id, o.quality, o.path, o.size, o.status, o.error, o.created_at, i.id, i.title,
	COALESCE((SELECT p.title FROM items p WHERE p.id = i.parent_id), ''),
	o.source_path, o.source_size, o.source_mtime, o.source_mtime_ns, o.job_id, o.audio_stream, o.audio_language, o.audio_title, o.duration_ms, o.bitrate`

func scanOptimized(row interface{ Scan(...any) error }, o *Optimized) error {
	return row.Scan(&o.FileID, &o.Quality, &o.Path, &o.Size, &o.Status, &o.Error, &o.CreatedAt, &o.ItemID, &o.Title, &o.SeriesTitle,
		&o.SourcePath, &o.SourceSize, &o.SourceMTime, &o.SourceMTimeNS, &o.JobID, &o.AudioStream, &o.AudioLanguage, &o.AudioTitle, &o.DurationMS, &o.Bitrate)
}

const optimizedFrom = ` FROM optimized_versions o JOIN item_files f ON f.file_id = o.file_id AND f.item_id=(SELECT MIN(representative.item_id) FROM item_files representative WHERE representative.file_id=o.file_id) JOIN items i ON i.id = f.item_id`

// MatchesSource compares a copy's provenance with the saved file revision. Filesystem freshness
// and artifact existence are checked separately, including at the final stream request.
func (o *Optimized) MatchesSource(f *MediaFile) bool {
	return f != nil && !f.Missing && o.FileID == f.ID && o.JobID != "" &&
		o.SourcePath == f.Path && o.SourceSize == f.Size && o.SourceMTime == f.ModTime
}

const optimizedSourceCurrent = `EXISTS (SELECT 1 FROM media_files m WHERE m.id=optimized_versions.file_id
	AND m.missing_since IS NULL AND m.path=source_path AND m.size=source_size AND m.mtime=source_mtime)`

// MatchesAudio requires the exact selected source track, including commentary tracks in
// the same language. Unknown legacy metadata must never be inferred from today's defaults.
func (o *Optimized) MatchesAudio(audio *Stream) bool {
	if audio == nil {
		return o.AudioStream == -1
	}
	return o.AudioStream >= 0 && o.AudioStream == audio.Index
}

// WithinBitrate uses the completed artifact's measured average total bitrate (bits/s).
// Unknown duration/rate is eligible only without a budget; this is not a peak/network guarantee.
func (o *Optimized) WithinBitrate(maxKbps int) bool {
	return maxKbps <= 0 || (o.DurationMS > 0 && o.Bitrate > 0 && (o.Bitrate-1)/1000+1 <= int64(maxKbps))
}

// QueueOptimize retains current completed/queued copies, but regenerates failed or changed ones.
func (s *Store) QueueOptimize(ctx context.Context, fileIDs []int64, quality int) (int, error) {
	n := 0
	for _, id := range fileIDs {
		res, err := s.db.ExecContext(ctx, `INSERT INTO optimized_versions
			(file_id, quality, status, created_at, source_path, source_size, source_mtime, job_id)
			SELECT id, ?, 'queued', ?, path, size, mtime, ? FROM media_files WHERE id=? AND missing_since IS NULL
			ON CONFLICT (file_id, quality) DO UPDATE SET status='queued', error='',
			created_at=excluded.created_at, source_path=excluded.source_path, source_size=excluded.source_size,
			source_mtime=excluded.source_mtime, source_mtime_ns=0, job_id=excluded.job_id, audio_stream=-2, audio_language='', audio_title='', duration_ms=0, bitrate=0
			WHERE optimized_versions.status='failed' OR source_path<>excluded.source_path
			OR source_size<>excluded.source_size OR source_mtime<>excluded.source_mtime`,
			quality, time.Now().Unix(), rand.Text(), id)
		if err != nil {
			return n, err
		}
		if k, _ := res.RowsAffected(); k > 0 {
			n++
		}
	}
	return n, nil
}

// NextOptimize returns the oldest queued copy, or nil.
func (s *Store) NextOptimize(ctx context.Context) (*Optimized, error) {
	var o Optimized
	err := scanOptimized(s.db.QueryRowContext(ctx, `SELECT `+optimizedCols+optimizedFrom+`
		WHERE o.status = 'queued' ORDER BY o.created_at, o.file_id LIMIT 1`), &o)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &o, err
}

// ClaimOptimize reserves a queued attempt only while its source still matches the catalog.
func (s *Store) ClaimOptimize(ctx context.Context, o *Optimized, sourceMTimeNS int64) (bool, error) {
	r, err := s.db.ExecContext(ctx, `UPDATE optimized_versions SET status='converting', source_mtime_ns=?
		WHERE file_id=? AND quality=? AND job_id=? AND status='queued' AND `+optimizedSourceCurrent,
		sourceMTimeNS, o.FileID, o.Quality, o.JobID)
	if err != nil {
		return false, err
	}
	n, err := r.RowsAffected()
	if n > 0 {
		o.SourceMTimeNS = sourceMTimeNS
	}
	return n > 0, err
}

// FinishOptimize cannot publish a deleted, superseded, or source-invalidated attempt.
func (s *Store) FinishOptimize(ctx context.Context, o *Optimized, path string, size int64) (bool, error) {
	r, err := s.db.ExecContext(ctx, `UPDATE optimized_versions SET status='done', path=?, size=?, error='', audio_stream=?, audio_language=?, audio_title=?, duration_ms=?, bitrate=?
		WHERE file_id=? AND quality=? AND job_id=? AND status='converting' AND source_mtime_ns=? AND `+optimizedSourceCurrent,
		path, size, o.AudioStream, o.AudioLanguage, o.AudioTitle, o.DurationMS, o.Bitrate, o.FileID, o.Quality, o.JobID, o.SourceMTimeNS)
	if err != nil {
		return false, err
	}
	n, err := r.RowsAffected()
	return n > 0, err
}

// FailOptimize retains an older artifact for removal and cannot overwrite a newer attempt.
func (s *Store) FailOptimize(ctx context.Context, o *Optimized, message string) (bool, error) {
	r, err := s.db.ExecContext(ctx, `UPDATE optimized_versions SET status='failed', error=?
		WHERE file_id=? AND quality=? AND job_id=?`, message, o.FileID, o.Quality, o.JobID)
	if err != nil {
		return false, err
	}
	n, err := r.RowsAffected()
	return n > 0, err
}

// ResetConverting puts copies left half-made (Arc stopped while converting) back in the queue.
func (s *Store) ResetConverting(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `UPDATE optimized_versions SET status='queued', source_mtime_ns=0,
		job_id=lower(hex(randomblob(16))) WHERE status='converting'`)
	return err
}

// FileOptimized lists a file's finished optimized copies, best first.
func (s *Store) FileOptimized(ctx context.Context, fileID int64) ([]Optimized, error) {
	return s.optimizedList(ctx, ` WHERE o.file_id=? AND o.status='done'
		AND EXISTS (SELECT 1 FROM media_files m WHERE m.id=o.file_id AND m.missing_since IS NULL
		AND m.path=o.source_path AND m.size=o.source_size AND m.mtime=o.source_mtime)
		ORDER BY o.quality DESC`, fileID)
}

// OptimizedVersions lists every optimized copy, newest first.
func (s *Store) OptimizedVersions(ctx context.Context) ([]Optimized, error) {
	return s.optimizedList(ctx, ` ORDER BY o.created_at DESC`)
}

func (s *Store) optimizedList(ctx context.Context, where string, args ...any) ([]Optimized, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+optimizedCols+optimizedFrom+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Optimized{}
	for rows.Next() {
		var o Optimized
		if err := scanOptimized(rows, &o); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// DeleteOptimized forgets an optimized copy and returns its file's path ("" if none), for deleting.
func (s *Store) DeleteOptimized(ctx context.Context, fileID int64, quality int) (string, error) {
	var path string
	err := s.db.QueryRowContext(ctx, `SELECT path FROM optimized_versions WHERE file_id = ? AND quality = ?`, fileID, quality).Scan(&path)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM optimized_versions WHERE file_id = ? AND quality = ?`, fileID, quality)
	return path, err
}
