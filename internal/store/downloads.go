package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"time"
)

var ErrDownloadNotFound = errors.New("download unavailable")
var ErrDownloadConflict = errors.New("download operation already used for another selection")
var ErrDownloadLimit = errors.New("download preparation record limit reached; release unused records")

// Download is a preparation descriptor, not an offline playback or transfer receipt.
// Private provenance must never be serialized or used as a caller-supplied path.
type Download struct {
	ID             string `json:"id"`
	ItemID         string `json:"item_id"`
	FileID         int64  `json:"file_id"`
	Revision       string `json:"revision"`
	State          string `json:"state"`
	ReasonCode     string `json:"reason_code,omitempty"`
	Size           int64  `json:"size"`
	SHA256         string `json:"sha256,omitempty"`
	CreatedAt      int64  `json:"created_at"`
	UserID         string `json:"-"`
	SessionHash    string `json:"-"`
	Key            string `json:"-"`
	SourcePath     string `json:"-"`
	SourceMTime    int64  `json:"-"`
	SourceMTimeNS  int64  `json:"-"`
	SourceCacheKey string `json:"-"`
	Snapshot       string `json:"-"`
}

const downloadCols = `id,item_id,file_id,revision,state,reason_code,source_size,sha256,created_at,user_id,session_hash,idempotency_key,source_path,source_mtime,source_mtime_ns,source_cache_key,snapshot`

func scanDownload(row interface{ Scan(...any) error }) (*Download, error) {
	var d Download
	err := row.Scan(&d.ID, &d.ItemID, &d.FileID, &d.Revision, &d.State, &d.ReasonCode, &d.Size, &d.SHA256, &d.CreatedAt, &d.UserID, &d.SessionHash, &d.Key, &d.SourcePath, &d.SourceMTime, &d.SourceMTimeNS, &d.SourceCacheKey, &d.Snapshot)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDownloadNotFound
	}
	return &d, err
}

// CreateDownload atomically enforces the record bound and idempotency scope. The caller has
// already checked selection/access; the worker independently rechecks them before publication.
func (s *Store) CreateDownload(ctx context.Context, user, session, item, key string, f MediaFile) (*Download, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	existing, err := scanDownload(tx.QueryRowContext(ctx, `SELECT `+downloadCols+` FROM downloads WHERE user_id=? AND session_hash=? AND idempotency_key=?`, user, session, key))
	if err == nil {
		if existing.ItemID != item || existing.FileID != f.ID {
			return nil, false, ErrDownloadConflict
		}
		return existing, false, nil
	}
	if !errors.Is(err, ErrDownloadNotFound) {
		return nil, false, err
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM downloads WHERE user_id=? AND session_hash=?`, user, session).Scan(&count); err != nil {
		return nil, false, err
	}
	if count >= 100 {
		return nil, false, ErrDownloadLimit
	}
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM downloads`).Scan(&count); err != nil {
		return nil, false, err
	}
	if count >= 2000 {
		return nil, false, ErrDownloadLimit
	}
	d := &Download{ID: rand.Text(), ItemID: item, FileID: f.ID, Revision: rand.Text(), State: "preparing", Size: f.Size, CreatedAt: time.Now().Unix(), UserID: user, SessionHash: session, Key: key, SourcePath: f.Path, SourceMTime: f.ModTime, SourceCacheKey: f.CacheKey}
	_, err = tx.ExecContext(ctx, `INSERT INTO downloads(id,item_id,file_id,revision,state,source_size,created_at,user_id,session_hash,idempotency_key,source_path,source_mtime,source_cache_key) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, d.ID, item, f.ID, d.Revision, d.State, f.Size, d.CreatedAt, user, session, key, f.Path, f.ModTime, f.CacheKey)
	if err != nil {
		return nil, false, err
	}
	return d, true, tx.Commit()
}
func (s *Store) Download(ctx context.Context, user, session, id string) (*Download, error) {
	return scanDownload(s.db.QueryRowContext(ctx, `SELECT `+downloadCols+` FROM downloads WHERE id=? AND user_id=? AND session_hash=?`, id, user, session))
}

// DownloadRecords is bounded by the atomic per-owner record limit. Permission filtering and
// pagination happen on this one bounded snapshot, without leaking hidden counts.
func (s *Store) DownloadRecords(ctx context.Context, user, session string) ([]Download, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+downloadCols+` FROM downloads WHERE user_id=? AND session_hash=? ORDER BY created_at,id LIMIT 100`, user, session)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Download{}
	for rows.Next() {
		d, err := scanDownload(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}

const downloadSourceFrom = ` FROM downloads d
      JOIN items i ON i.id=d.item_id
      JOIN media_files m ON m.id=d.file_id
      JOIN item_files link ON link.item_id=d.item_id AND link.file_id=d.file_id
      JOIN sessions se ON se.token_hash=d.session_hash AND se.user_id=d.user_id
      WHERE d.user_id=? AND d.session_hash=? AND m.missing_since IS NULL
        AND d.source_cache_key<>'' AND m.cache_key=d.source_cache_key
        AND m.path=d.source_path AND m.size=d.source_size AND m.mtime=d.source_mtime
        AND link.part<=1
        AND (SELECT count(*) FROM item_files shared WHERE shared.file_id=d.file_id)=1
        AND NOT EXISTS (SELECT 1 FROM item_files parts WHERE parts.item_id=d.item_id AND parts.part>1)`

// VisibleDownloadRecords is a narrow, bounded descriptor projection. Unlike preparation/get,
// a list needs neither full item metadata nor media streams. Check current owner/grant,
// source revision and topology in one set query; keep execution-time checks in DownloadSource.
func (s *Store) VisibleDownloadRecords(ctx context.Context, user, session string) ([]Download, error) {
	u, err := s.GetUser(ctx, user)
	if err != nil {
		return nil, err
	}
	if u == nil || (!u.IsAdmin && !u.AllowDownloads) {
		return []Download{}, nil
	}
	acc := Access{}
	if !u.IsAdmin {
		acc = Access{MaxAge: u.MaxAge, Libraries: u.LibraryIDs, ExcludeLabels: u.ExcludeLabels}
	}
	query := `SELECT ` + prefixCols("d.", downloadCols) + downloadSourceFrom
	query, args := acc.Where("i", query, []any{user, session})
	query += ` ORDER BY d.created_at,d.id LIMIT 100`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Download{}
	for rows.Next() {
		d, err := scanDownload(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}

func (s *Store) NextDownload(ctx context.Context) (*Download, error) {
	return scanDownload(s.db.QueryRowContext(ctx, `SELECT `+downloadCols+` FROM downloads WHERE state='preparing' ORDER BY created_at,id LIMIT 1`))
}

// DownloadSource verifies current ownership, grant, visibility and exact single-item file linkage.
// It is shared by reads and preparation; absence/revocation never exposes private details.
func (s *Store) DownloadSource(ctx context.Context, d *Download) (*MediaFile, error) {
	u, err := s.GetUser(ctx, d.UserID)
	if err != nil {
		return nil, err
	}
	if u == nil || (!u.IsAdmin && !u.AllowDownloads) {
		return nil, ErrDownloadNotFound
	}
	acc := Access{}
	if !u.IsAdmin {
		acc = Access{MaxAge: u.MaxAge, Libraries: u.LibraryIDs, ExcludeLabels: u.ExcludeLabels}
	}
	query := `SELECT m.id,m.library_id,m.path,m.size,m.mtime,m.cache_key,m.container,m.duration_ms,m.bitrate` + downloadSourceFrom + ` AND d.id=? AND d.revision=? AND d.source_cache_key=?`
	query, args := acc.Where("i", query, []any{d.UserID, d.SessionHash, d.ID, d.Revision, d.SourceCacheKey})
	var f MediaFile
	err = s.db.QueryRowContext(ctx, query, args...).Scan(&f.ID, &f.LibraryID, &f.Path, &f.Size, &f.ModTime, &f.CacheKey, &f.Container, &f.DurationMS, &f.Bitrate)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDownloadNotFound
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (s *Store) FinishDownload(ctx context.Context, d *Download, digest string, mtimeNS int64, reason string) (bool, error) {
	return s.FinishDownloadBundle(ctx, d, digest, mtimeNS, reason, "")
}

// FinishDownloadBundle publishes the digest and complete offline snapshot in the same attempt CAS.
func (s *Store) FinishDownloadBundle(ctx context.Context, d *Download, digest string, mtimeNS int64, reason, snapshot string) (bool, error) {
	state := "ready"
	if reason != "" {
		state = "failed"
		snapshot = ""
		digest = ""
		mtimeNS = 0
	}
	result, err := s.db.ExecContext(ctx, `UPDATE downloads SET state=?,reason_code=?,sha256=?,source_mtime_ns=?,snapshot=? WHERE id=? AND revision=? AND state='preparing' AND source_cache_key=?
 AND (?<>'' OR EXISTS (SELECT 1 FROM media_files m WHERE m.id=downloads.file_id AND m.cache_key=downloads.source_cache_key))`, state, reason, digest, mtimeNS, snapshot, d.ID, d.Revision, d.SourceCacheKey, reason)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n > 0, err
}
func (s *Store) DeleteDownload(ctx context.Context, user, session, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM downloads WHERE id=? AND user_id=? AND session_hash=?`, id, user, session)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return ErrDownloadNotFound
	}
	return err
}

// DownloadBundleRefs lists only published bundles for root-owned orphan reconciliation.
func (s *Store) DownloadBundleRefs(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,revision FROM downloads WHERE state='ready' AND snapshot<>''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	refs := map[string]string{}
	for rows.Next() {
		var id, revision string
		if err = rows.Scan(&id, &revision); err != nil {
			return nil, err
		}
		refs[id] = revision
	}
	return refs, rows.Err()
}
