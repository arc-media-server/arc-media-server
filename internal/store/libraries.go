package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// SyncLibrary creates or updates a library by name and replaces its folder list.
func (s *Store) SyncLibrary(ctx context.Context, name, kind, sourceOrdering string, paths []string) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Other videos libraries are stored as movies with a flag (see migration 0034).
	videos := kind == "videos"
	if videos {
		kind = "movies"
	}
	var id int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO libraries (name, kind, source_ordering, videos, created_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (name) DO UPDATE SET kind = excluded.kind, source_ordering = excluded.source_ordering, videos = excluded.videos
		RETURNING id`, name, kind, sourceOrdering, videos, time.Now().Unix()).Scan(&id)
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM library_paths WHERE library_id = ?`, id); err != nil {
		return 0, err
	}
	for _, p := range paths {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO library_paths (library_id, path) VALUES (?, ?)`, id, p); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// ErrDuplicateName means another library already has the name.
var ErrDuplicateName = errors.New("a library with that name already exists")

// CreateLibrary adds a library. It returns ErrDuplicateName if the name is taken.
func (s *Store) CreateLibrary(ctx context.Context, name, kind, sourceOrdering string, paths []string) (int64, error) {
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM libraries WHERE name = ? COLLATE NOCASE`, name).Scan(&exists); err != nil {
		return 0, err
	}
	if exists > 0 {
		return 0, ErrDuplicateName
	}
	return s.SyncLibrary(ctx, name, kind, sourceOrdering, paths)
}

// UpdateLibrary renames a library, replaces folders and sets file numbering. An optional
// provider is saved in the same transaction, including duplicate-name validation.
// Files that were in a folder no longer in the library are forgotten (their titles are kept, so
// history survives, but no longer listed). It returns ErrDuplicateName if the name is taken.
func (s *Store) UpdateLibrary(ctx context.Context, id int64, name, sourceOrdering string, paths []string, detailsProvider ...string) error {
	var provider any
	if len(detailsProvider) > 0 {
		provider = detailsProvider[0]
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM libraries WHERE name = ? COLLATE NOCASE AND id <> ?`,
		name, id).Scan(&exists); err != nil {
		return err
	}
	if exists > 0 {
		return ErrDuplicateName
	}
	if _, err := tx.ExecContext(ctx, `UPDATE libraries SET name = ?, source_ordering = ?, details_provider = COALESCE(?, details_provider) WHERE id = ?`,
		name, sourceOrdering, provider, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM library_paths WHERE library_id = ?`, id); err != nil {
		return err
	}
	for _, p := range paths {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO library_paths (library_id, path) VALUES (?, ?)`, id, p); err != nil {
			return err
		}
	}

	// Forget files outside the remaining folders.
	rows, err := tx.QueryContext(ctx, `SELECT id, path FROM media_files WHERE library_id = ?`, id)
	if err != nil {
		return err
	}
	var gone []int64
	for rows.Next() {
		var fid int64
		var p string
		if err := rows.Scan(&fid, &p); err != nil {
			rows.Close()
			return err
		}
		if !underAny(p, paths) {
			gone = append(gone, fid)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, fid := range gone {
		if _, err := tx.ExecContext(ctx, `DELETE FROM media_files WHERE id = ?`, fid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DeleteLibrary removes a library and everything AMS knows about it. Media files aren't touched.
func (s *Store) DeleteLibrary(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM libraries WHERE id = ?`, id)
	return err
}

// LibraryCounts returns how many movies or shows each library lists (as ListItems counts them).
func (s *Store) LibraryCounts(ctx context.Context) (map[int64]int, error) {
	libs, err := s.ListLibraries(ctx)
	if err != nil {
		return nil, err
	}
	out := map[int64]int{}
	for _, l := range libs {
		kind := "movie"
		if l.Kind == "shows" {
			kind = "series"
		}
		_, n, err := s.ListItems(ctx, ItemQuery{LibraryID: l.ID, Kind: kind, Limit: 1})
		if err != nil {
			return nil, err
		}
		out[l.ID] = n
	}
	return out, nil
}

// underAny reports whether path is one of folders or inside one of them.
func underAny(path string, folders []string) bool {
	for _, f := range folders {
		f = strings.TrimRight(f, `/\`)
		if path == f || strings.HasPrefix(path, f+`\`) || strings.HasPrefix(path, f+`/`) {
			return true
		}
	}
	return false
}

func (s *Store) ListLibraries(ctx context.Context) ([]Library, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, CASE WHEN videos = 1 THEN 'videos' ELSE kind END, source_ordering, trickplay, collection_min, metadata_language, skip_intros, skip_credits, collection_grid, details_provider FROM libraries ORDER BY name`)
	if err != nil {
		return nil, err
	}
	var libs []Library
	for rows.Next() {
		var l Library
		if err := rows.Scan(&l.ID, &l.Name, &l.Kind, &l.SourceOrdering, &l.Trickplay, &l.CollectionMin, &l.MetadataLanguage, &l.SkipIntros, &l.SkipCredits, &l.CollectionGrid, &l.DetailsProvider); err != nil {
			rows.Close()
			return nil, err
		}
		libs = append(libs, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range libs {
		if libs[i].Paths, err = s.libraryPaths(ctx, libs[i].ID); err != nil {
			return nil, err
		}
	}
	return libs, nil
}

// GetLibrary returns nil when no library has the given id.
func (s *Store) GetLibrary(ctx context.Context, id int64) (*Library, error) {
	var l Library
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, CASE WHEN videos = 1 THEN 'videos' ELSE kind END, source_ordering, trickplay, collection_min, metadata_language, skip_intros, skip_credits, collection_grid, details_provider FROM libraries WHERE id = ?`, id).
		Scan(&l.ID, &l.Name, &l.Kind, &l.SourceOrdering, &l.Trickplay, &l.CollectionMin, &l.MetadataLanguage, &l.SkipIntros, &l.SkipCredits, &l.CollectionGrid, &l.DetailsProvider)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if l.Paths, err = s.libraryPaths(ctx, id); err != nil {
		return nil, err
	}
	return &l, nil
}

func (s *Store) libraryPaths(ctx context.Context, id int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT path FROM library_paths WHERE library_id = ? ORDER BY path`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	paths := []string{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

// SetLibraryTrickplay turns a library's timeline thumbnails on or off.
func (s *Store) SetLibraryTrickplay(ctx context.Context, id int64, on bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE libraries SET trickplay = ? WHERE id = ?`, boolInt(on), id)
	return err
}

// LibraryFileIDs lists the ids of a library's files.
func (s *Store) LibraryFileIDs(ctx context.Context, id int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM media_files WHERE library_id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var fid int64
		if err := rows.Scan(&fid); err != nil {
			return nil, err
		}
		ids = append(ids, fid)
	}
	return ids, rows.Err()
}

// SetLibraryLanguage changes the language a library's metadata is fetched in, and has every movie
// and show fetched again over the next scans.
func (s *Store) SetLibraryLanguage(ctx context.Context, id int64, lang string) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE libraries SET metadata_language = ? WHERE id = ?`, lang, id); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE items SET synced_at = 0, metadata_state = 'pending' WHERE library_id = ? AND kind IN ('movie', 'series') AND metadata_state <> 'locked'`, id)
	return err
}

// SetLibrarySkips sets whether a library offers Skip Intro and Skip Credits.
func (s *Store) SetLibrarySkips(ctx context.Context, id int64, intros, credits bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE libraries SET skip_intros = ?, skip_credits = ? WHERE id = ?`, boolInt(intros), boolInt(credits), id)
	return err
}

// SetLibraryCollectionGrid sets how a library's collections show in its grid by default.
func (s *Store) SetLibraryCollectionGrid(ctx context.Context, id int64, mode string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE libraries SET collection_grid = ? WHERE id = ?`, mode, id)
	return err
}

// SetLibraryDetailsProvider chooses show-level information independently of file numbering.
func (s *Store) SetLibraryDetailsProvider(ctx context.Context, id int64, provider string) error {
	if provider != "" && provider != "tvdb" {
		return errors.New("metadata provider must be TMDB or TVDB")
	}
	_, err := s.db.ExecContext(ctx, "UPDATE libraries SET details_provider = ? WHERE id = ?", provider, id)
	return err
}
