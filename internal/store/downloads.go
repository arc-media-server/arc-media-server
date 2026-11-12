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
	ID            string `json:"id"`
	ItemID        string `json:"item_id"`
	FileID        int64  `json:"file_id"`
	Revision      string `json:"revision"`
	State         string `json:"state"`
	ReasonCode    string `json:"reason_code,omitempty"`
	Size          int64  `json:"size"`
	SHA256        string `json:"sha256,omitempty"`
	CreatedAt     int64  `json:"created_at"`
	UserID        string `json:"-"`
	SessionHash   string `json:"-"`
	Key           string `json:"-"`
	SourcePath    string `json:"-"`
	SourceMTime   int64  `json:"-"`
	SourceMTimeNS int64  `json:"-"`
}

const downloadCols = `id,item_id,file_id,revision,state,reason_code,source_size,sha256,created_at,user_id,session_hash,idempotency_key,source_path,source_mtime,source_mtime_ns`

func scanDownload(row interface{ Scan(...any) error }) (*Download, error) {
	var d Download
	err := row.Scan(&d.ID, &d.ItemID, &d.FileID, &d.Revision, &d.State, &d.ReasonCode, &d.Size, &d.SHA256, &d.CreatedAt, &d.UserID, &d.SessionHash, &d.Key, &d.SourcePath, &d.SourceMTime, &d.SourceMTimeNS)
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
	d := &Download{ID: rand.Text(), ItemID: item, FileID: f.ID, Revision: rand.Text(), State: "preparing", Size: f.Size, CreatedAt: time.Now().Unix(), UserID: user, SessionHash: session, Key: key, SourcePath: f.Path, SourceMTime: f.ModTime}
	_, err = tx.ExecContext(ctx, `INSERT INTO downloads(id,item_id,file_id,revision,state,source_size,created_at,user_id,session_hash,idempotency_key,source_path,source_mtime) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, d.ID, item, f.ID, d.Revision, d.State, f.Size, d.CreatedAt, user, session, key, f.Path, f.ModTime)
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
func (s *Store) NextDownload(ctx context.Context) (*Download, error) {
	return scanDownload(s.db.QueryRowContext(ctx, `SELECT `+downloadCols+` FROM downloads WHERE state='preparing' ORDER BY created_at,id LIMIT 1`))
}

// DownloadSource verifies current ownership, grant, visibility and exact single-item file linkage.
// It is shared by reads and preparation; absence/revocation never exposes private details.
func (s *Store) DownloadSource(ctx context.Context, d *Download) (*MediaFile, error) {
	current, err := s.Download(ctx, d.UserID, d.SessionHash, d.ID)
	if err != nil {
		return nil, err
	}
	if current.Revision != d.Revision {
		return nil, ErrDownloadNotFound
	}
	se, err := s.GetSession(ctx, d.SessionHash)
	if err != nil {
		return nil, err
	}
	if se == nil || se.UserID != d.UserID {
		return nil, ErrDownloadNotFound
	}
	u, err := s.GetUser(ctx, d.UserID)
	if err != nil {
		return nil, err
	}
	if u == nil || (!u.IsAdmin && !u.AllowDownloads) {
		return nil, ErrDownloadNotFound
	}
	it, err := s.GetItem(ctx, d.ItemID)
	if err != nil {
		return nil, err
	}
	if it == nil {
		return nil, ErrDownloadNotFound
	}
	acc := Access{}
	if !u.IsAdmin {
		acc = Access{MaxAge: u.MaxAge, Libraries: u.LibraryIDs, ExcludeLabels: u.ExcludeLabels}
	}
	ok, err := s.CanSee(ctx, it, acc)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrDownloadNotFound
	}
	files, err := s.ItemFiles(ctx, d.ItemID)
	if err != nil {
		return nil, err
	}
	var f *MediaFile
	for i := range files {
		if files[i].ID == d.FileID {
			f = &files[i]
		}
	}
	if f == nil || f.Missing || f.Path != d.SourcePath || f.Size != d.Size || f.ModTime != d.SourceMTime {
		return nil, ErrDownloadNotFound
	}
	links, err := s.FileLinks(ctx, []int64{d.FileID})
	if err != nil {
		return nil, err
	}
	for _, part := range files {
		if part.Part > 1 {
			return nil, ErrDownloadNotFound
		}
	}
	if len(links[d.FileID]) != 1 || f.Part > 1 {
		return nil, ErrDownloadNotFound
	}
	return f, nil
}
func (s *Store) FinishDownload(ctx context.Context, d *Download, digest string, mtimeNS int64, reason string) (bool, error) {
	state := "ready"
	if reason != "" {
		state = "failed"
		digest = ""
		mtimeNS = 0
	}
	result, err := s.db.ExecContext(ctx, `UPDATE downloads SET state=?,reason_code=?,sha256=?,source_mtime_ns=? WHERE id=? AND revision=? AND state='preparing'`, state, reason, digest, mtimeNS, d.ID, d.Revision)
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
