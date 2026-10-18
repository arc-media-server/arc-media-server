package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Optimized versions: easy-to-play copies of files (H.264 and AAC in MP4, at most 1080p or 720p),
// made ahead of time in Arc's data folder so weak or remote players needn't convert live.

// Optimized is one optimized copy of a file.
type Optimized struct {
	FileID  int64  `json:"file_id"`
	Quality int    `json:"quality"` // the most lines tall: 1080 or 720
	Path    string `json:"-"`
	Size    int64  `json:"size"`
	// queued, converting, done or failed.
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	// What it's of, for the list in Settings.
	ItemID      string `json:"item_id"`
	Title       string `json:"title"`
	SeriesTitle string `json:"series_title,omitempty"`
	CreatedAt   int64  `json:"created_at"`
}

const optimizedCols = `o.file_id, o.quality, o.path, o.size, o.status, o.error, o.created_at, i.id, i.title,
	COALESCE((SELECT p.title FROM items p WHERE p.id = i.parent_id), '')`

func scanOptimized(row interface{ Scan(...any) error }, o *Optimized) error {
	return row.Scan(&o.FileID, &o.Quality, &o.Path, &o.Size, &o.Status, &o.Error, &o.CreatedAt, &o.ItemID, &o.Title, &o.SeriesTitle)
}

const optimizedFrom = ` FROM optimized_versions o JOIN item_files f ON f.file_id = o.file_id JOIN items i ON i.id = f.item_id`

// QueueOptimize asks for optimized copies of files at a quality (ones already made or queued stay).
func (s *Store) QueueOptimize(ctx context.Context, fileIDs []int64, quality int) (int, error) {
	n := 0
	for _, id := range fileIDs {
		res, err := s.db.ExecContext(ctx, `INSERT INTO optimized_versions (file_id, quality, status, created_at)
			VALUES (?, ?, 'queued', ?) ON CONFLICT (file_id, quality) DO UPDATE SET status = 'queued', error = ''
			WHERE status = 'failed'`, id, quality, time.Now().Unix())
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

// SetOptimizeStatus records how making a copy is going.
func (s *Store) SetOptimizeStatus(ctx context.Context, fileID int64, quality int, status, path string, size int64, errText string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE optimized_versions SET status = ?, path = ?, size = ?, error = ?
		WHERE file_id = ? AND quality = ?`, status, path, size, errText, fileID, quality)
	return err
}

// ResetConverting puts copies left half-made (Arc stopped while converting) back in the queue.
func (s *Store) ResetConverting(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `UPDATE optimized_versions SET status = 'queued' WHERE status = 'converting'`)
	return err
}

// FileOptimized lists a file's finished optimized copies, best first.
func (s *Store) FileOptimized(ctx context.Context, fileID int64) ([]Optimized, error) {
	return s.optimizedList(ctx, ` WHERE o.file_id = ? AND o.status = 'done' ORDER BY o.quality DESC`, fileID)
}

// OptimizedVersions lists every optimized copy, newest first.
func (s *Store) OptimizedVersions(ctx context.Context) ([]Optimized, error) {
	return s.optimizedList(ctx, ` GROUP BY o.file_id, o.quality ORDER BY o.created_at DESC`)
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
