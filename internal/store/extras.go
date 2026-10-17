package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
)

// Extra is a trailer, featurette or similar that belongs to a movie or show.
type Extra struct {
	ID         string `json:"id"`
	ItemID     string `json:"item_id"`
	FileID     int64  `json:"-"`
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	DurationMS int64  `json:"duration_ms,omitempty"`
	Path       string `json:"-"`
}

// SaveExtra records that a file is an extra of an item (keeping its id if it was already known).
func (s *Store) SaveExtra(ctx context.Context, itemID string, fileID int64, kind, title string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO extras (id, item_id, file_id, kind, title) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (file_id) DO UPDATE SET item_id = excluded.item_id, kind = excluded.kind, title = excluded.title`,
		NewID(), itemID, fileID, kind, title)
	return err
}

// ItemExtras lists an item's extras whose files are present: trailers first, then by title.
func (s *Store) ItemExtras(ctx context.Context, itemID string) ([]Extra, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT x.id, x.item_id, x.file_id, x.kind, x.title, m.duration_ms, m.path FROM extras x
		JOIN media_files m ON m.id = x.file_id
		WHERE x.item_id = ? AND m.missing_since IS NULL
		ORDER BY x.kind <> 'trailer', x.title`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Extra{}
	for rows.Next() {
		var e Extra
		if err := rows.Scan(&e.ID, &e.ItemID, &e.FileID, &e.Kind, &e.Title, &e.DurationMS, &e.Path); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// GetExtra returns nil when there's no such extra.
func (s *Store) GetExtra(ctx context.Context, id string) (*Extra, error) {
	var e Extra
	err := s.db.QueryRowContext(ctx, `
		SELECT x.id, x.item_id, x.file_id, x.kind, x.title, m.duration_ms, m.path FROM extras x
		JOIN media_files m ON m.id = x.file_id WHERE x.id = ?`, id).
		Scan(&e.ID, &e.ItemID, &e.FileID, &e.Kind, &e.Title, &e.DurationMS, &e.Path)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// filesBelow lists present files under dir (at any depth) in a library with the movie or episode
// they belong to: (path, item id, item kind, parent id).
func (s *Store) filesBelow(ctx context.Context, libraryID int64, dir string) ([][4]string, error) {
	dir = strings.TrimRight(dir, `/\`)
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.path, i.id, i.kind, COALESCE(i.parent_id, '') FROM media_files m
		JOIN item_files f ON f.file_id = m.id JOIN items i ON i.id = f.item_id
		WHERE m.library_id = ? AND m.missing_since IS NULL AND substr(m.path, 1, ?) = ?`,
		libraryID, len(dir)+1, dir+string(filepath.Separator))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][4]string
	for rows.Next() {
		var r [4]string
		if err := rows.Scan(&r[0], &r[1], &r[2], &r[3]); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// OwnerInFolder finds the movie whose file is directly in dir, or the show with an episode file
// anywhere below dir. It returns "" when there's none, or more than one.
func (s *Store) OwnerInFolder(ctx context.Context, libraryID int64, kind, dir string) (string, error) {
	files, err := s.filesBelow(ctx, libraryID, dir)
	if err != nil {
		return "", err
	}
	owners := map[string]bool{}
	for _, f := range files {
		switch {
		case kind == "movies" && f[2] == "movie" && filepath.Dir(f[0]) == filepath.Clean(dir):
			owners[f[1]] = true
		case kind == "shows" && f[2] == "episode":
			owners[f[3]] = true
		}
	}
	if len(owners) != 1 {
		return "", nil
	}
	for id := range owners {
		return id, nil
	}
	return "", nil
}

// MovieByFileStem finds the movie whose file is dir/stem.<ext>, or "".
func (s *Store) MovieByFileStem(ctx context.Context, libraryID int64, dir, stem string) (string, error) {
	files, err := s.filesBelow(ctx, libraryID, dir)
	if err != nil {
		return "", err
	}
	for _, f := range files {
		base := filepath.Base(f[0])
		if f[2] == "movie" && filepath.Dir(f[0]) == filepath.Clean(dir) &&
			strings.EqualFold(strings.TrimSuffix(base, filepath.Ext(base)), stem) {
			return f[1], nil
		}
	}
	return "", nil
}
