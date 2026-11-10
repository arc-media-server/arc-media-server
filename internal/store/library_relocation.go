package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
)

type libraryRelocation struct {
	oldRoot, newRoot string
	files            []relocatedFile
}
type relocatedFile struct {
	id, size, mtime, mtimeNS int64
	oldPath, newPath         string
}

// prepareLibraryRelocation checks destination revisions before taking the writer.
// Only one disjoint root replaced by one root has an unambiguous prefix mapping.
// No directory walk, media read, rename or write is performed.
func (s *Store) prepareLibraryRelocation(ctx context.Context, id int64, paths []string) (*libraryRelocation, error) {
	if len(paths) != 1 {
		return nil, nil
	}
	lib, err := s.GetLibrary(ctx, id)
	if err != nil || lib == nil {
		return nil, err
	}
	if len(lib.Paths) != 1 {
		return nil, nil
	}
	oldRoot, newRoot := filepath.Clean(lib.Paths[0]), filepath.Clean(paths[0])
	if !filepath.IsAbs(oldRoot) || !filepath.IsAbs(newRoot) || underAny(oldRoot, []string{newRoot}) || underAny(newRoot, []string{oldRoot}) {
		return nil, nil
	}
	plan := &libraryRelocation{oldRoot: oldRoot, newRoot: newRoot}
	rows, err := s.db.QueryContext(ctx, `SELECT id,path,size,mtime FROM media_files WHERE library_id=?`, id)
	if err != nil {
		return nil, err
	}
	var saved []relocatedFile
	for rows.Next() {
		var f relocatedFile
		if err := rows.Scan(&f.id, &f.oldPath, &f.size, &f.mtime); err != nil {
			rows.Close()
			return nil, err
		}
		saved = append(saved, f)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, f := range saved {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		next, ok := relocatedPath(f.oldPath, oldRoot, newRoot)
		if !ok || f.size <= 0 || f.mtime <= 0 {
			continue
		}
		info, err := os.Stat(next)
		if err != nil || !info.Mode().IsRegular() || info.Size() != f.size || info.ModTime().Unix() != f.mtime {
			continue
		}
		f.newPath = next
		f.mtimeNS = info.ModTime().UnixNano()
		plan.files = append(plan.files, f)
	}
	return plan, nil
}

func relocatedPath(path, oldRoot, newRoot string) (string, bool) {
	rel, err := filepath.Rel(oldRoot, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", false
	}
	return filepath.Join(newRoot, rel), true
}

// applyLibraryRelocationTx rechecks database identity and destination collisions.
// A concurrent revision or ambiguous target is retained as missing, never guessed.
func applyLibraryRelocationTx(ctx context.Context, tx *sql.Tx, id int64, plan *libraryRelocation) error {
	if plan == nil {
		return nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT path FROM library_paths WHERE library_id=?`, id)
	if err != nil {
		return err
	}
	var roots []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			return err
		}
		roots = append(roots, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(roots) != 1 || filepath.Clean(roots[0]) != plan.oldRoot {
		return nil
	}
	for _, f := range plan.files {
		result, err := tx.ExecContext(ctx, `UPDATE media_files SET path=?,missing_since=NULL WHERE id=? AND library_id=? AND path=? AND size=? AND mtime=? AND NOT EXISTS(SELECT 1 FROM media_files WHERE path=?)`, f.newPath, f.id, id, f.oldPath, f.size, f.mtime, f.newPath)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			continue
		}
		// Completed copies require nanosecond provenance too. Never let an old
		// in-flight encoder publish against the relocated file's identity.
		if _, err := tx.ExecContext(ctx, `UPDATE optimized_versions SET source_path=? WHERE file_id=? AND status='done' AND source_path=? AND source_size=? AND source_mtime=? AND source_mtime_ns=?`, f.newPath, f.id, f.oldPath, f.size, f.mtime, f.mtimeNS); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE optimized_versions SET status='failed',error='The source moved. Optimize it again after scanning.',job_id=lower(hex(randomblob(16))) WHERE file_id=? AND status<>'failed' AND (status<>'done' OR source_path<>?)`, f.id, f.newPath); err != nil {
			return err
		}
	}
	return nil
}
