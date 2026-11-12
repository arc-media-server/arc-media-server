package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type BackgroundJob struct {
	State, Error string
	UpdatedAt    int64
	// A done job asked to run again at RetryAt (0 for never); Retries is how many times it has.
	RetryAt, Retries int64
}

// BackgroundRevision ties completion to the exact saved file revisions; a new episode
// changes a season key too. Never read media here or run discovery to find pending work.
func (s *Store) BackgroundRevision(ctx context.Context, kind string, ids []int64) (revision string, newest, maximum int64, err error) {
	var parts []string
	for _, id := range ids {
		var size, mtime, probed, generation int64
		var path string
		if err = s.db.QueryRowContext(ctx, `SELECT size,mtime,COALESCE(probed_at,0),local_marker_generation,path FROM media_files WHERE id=? AND missing_since IS NULL`, id).Scan(&size, &mtime, &probed, &generation, &path); err != nil {
			return
		}
		part := fmt.Sprintf("%d:%d:%d:%d:%s", id, size, mtime, len(path), path)
		if kind == "markers" {
			part += fmt.Sprintf(":%d", generation)
		}
		parts = append(parts, part)
		newest = max(newest, probed)
		maximum = max(maximum, id)
	}
	revision = strings.Join(parts, ";")
	return
}
func (s *Store) BackgroundJob(ctx context.Context, kind, key, revision string, fileID int64) (BackgroundJob, error) {
	var existing BackgroundJob
	var oldRevision string
	if e := s.db.QueryRowContext(ctx, `SELECT revision,state,error,updated_at,retry_at,retries FROM background_jobs WHERE kind=? AND job_key=?`, kind, key).Scan(&oldRevision, &existing.State, &existing.Error, &existing.UpdatedAt, &existing.RetryAt, &existing.Retries); e == nil && oldRevision == revision {
		return existing, nil
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO background_jobs(kind,job_key,revision,file_id) VALUES(?,?,?,?)
 ON CONFLICT(kind,job_key) DO UPDATE SET revision=excluded.revision,file_id=excluded.file_id,state='queued',error='',updated_at=0,retry_at=0,retries=0 WHERE background_jobs.revision<>excluded.revision`, kind, key, revision, fileID)
	if err != nil {
		return BackgroundJob{}, err
	}
	var j BackgroundJob
	err = s.db.QueryRowContext(ctx, `SELECT state,error,updated_at,retry_at,retries FROM background_jobs WHERE kind=? AND job_key=?`, kind, key).Scan(&j.State, &j.Error, &j.UpdatedAt, &j.RetryAt, &j.Retries)
	return j, err
}

// RetryBackgroundJobAt asks a done job to run again at the given time, counting the retry. It only
// applies to the same revision: changed files start afresh (no retries counted).
func (s *Store) RetryBackgroundJobAt(ctx context.Context, kind, key, revision string, at int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE background_jobs SET retry_at=?,retries=retries+1 WHERE kind=? AND job_key=? AND revision=? AND state='done'`, at, kind, key, revision)
	return err
}

// ReopenBackgroundRetry queues a done job whose retry time has come, keeping its retry count.
func (s *Store) ReopenBackgroundRetry(ctx context.Context, kind, key, revision string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE background_jobs SET state='queued',error='',retry_at=0 WHERE kind=? AND job_key=? AND revision=? AND state='done'`, kind, key, revision)
	return err
}
func (s *Store) SetBackgroundJob(ctx context.Context, kind, key, revision, state, message string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE background_jobs SET state=?,error=?,updated_at=? WHERE kind=? AND job_key=? AND revision=?`, state, message, time.Now().Unix(), kind, key, revision)
	return err
}
func (s *Store) RecoverBackgroundJobs(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `UPDATE background_jobs SET state='queued',error='' WHERE state='running'`)
	return err
}
func (s *Store) RetryBackgroundJobs(ctx context.Context, kind string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE background_jobs SET state='queued',error='',updated_at=0 WHERE kind=? AND state='failed'`, kind)
	return err
}
func (s *Store) BackgroundCounts(ctx context.Context, kind string) (pending, done, failed int, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(state IN ('queued','paused','running')),0),COALESCE(SUM(state='done'),0),COALESCE(SUM(state='failed'),0) FROM background_jobs WHERE kind=?`, kind).Scan(&pending, &done, &failed)
	return
}
func (s *Store) MaxFileID(ctx context.Context) (id int64, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(id),0) FROM media_files`).Scan(&id)
	return
}

// PendingFormatPage is a cursor page, never a run limit. Completed attempts remain excluded
// by their persistent version; offline entries cannot keep later IDs out of the next page.
// Every video file read before FormatVersion qualifies (aspect ratios came with version 2);
// the frame sample for HDR10+/Dolby Vision still only runs on likely HDR streams.
func (s *Store) PendingFormatPage(ctx context.Context, libraryID, after int64) ([]MediaFile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,path,size,mtime FROM media_files m WHERE library_id=? AND id>? AND format_probe_version<? AND probed_at IS NOT NULL AND probe_error='' AND missing_since IS NULL
 AND EXISTS(SELECT 1 FROM media_streams st WHERE st.file_id=m.id AND st.kind='video') ORDER BY id LIMIT 32`, libraryID, after, FormatVersion)
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
