package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Playlist is one profile's list of movies and episodes, played in order.
type Playlist struct {
	ID        string `json:"id"`
	UserID    string `json:"-"`
	Title     string `json:"title"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
	// A smart playlist's filter (JSON); "" for a list of entries.
	Filter string `json:"-"`
	// Filled in when listing.
	Count      int   `json:"count"`
	DurationMS int64 `json:"duration_ms"`
}

// PlaylistEntry is a title's place in a playlist.
type PlaylistEntry struct {
	ID     int64
	ItemID string
}

const playlistCols = `id, user_id, title, created_at, updated_at, filter`

func scanPlaylist(row interface{ Scan(...any) error }, p *Playlist) error {
	return row.Scan(&p.ID, &p.UserID, &p.Title, &p.CreatedAt, &p.UpdatedAt, &p.Filter)
}

// Playlists lists a profile's playlists, most recently changed first, with how many titles each
// has (that the profile can see) and how long they run.
func (s *Store) Playlists(ctx context.Context, userID string, acc Access) ([]Playlist, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+playlistCols+` FROM playlists WHERE user_id = ? ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	out := []Playlist{}
	for rows.Next() {
		var p Playlist
		if err := scanPlaylist(rows, &p); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	byID := make(map[string]int, len(out))
	for i := range out {
		byID[out[i].ID] = i
	}
	q := "SELECT e.playlist_id, COUNT(*), COALESCE(SUM(items.runtime_min),0)*60000 FROM playlist_entries e JOIN playlists p ON p.id=e.playlist_id JOIN items ON items.id=e.item_id WHERE p.user_id=? AND " + playableSQL
	q, args := acc.Where("items", q, []any{userID})
	counts, err := s.db.QueryContext(ctx, q+" GROUP BY e.playlist_id", args...)
	if err != nil {
		return nil, err
	}
	defer counts.Close()
	for counts.Next() {
		var id string
		var count int
		var duration int64
		if err := counts.Scan(&id, &count, &duration); err != nil {
			return nil, err
		}
		if i, ok := byID[id]; ok {
			out[i].Count, out[i].DurationMS = count, duration
		}
	}
	if err := counts.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// GetPlaylist returns a playlist, or nil.
func (s *Store) GetPlaylist(ctx context.Context, id string) (*Playlist, error) {
	var p Playlist
	err := scanPlaylist(s.db.QueryRowContext(ctx, `SELECT `+playlistCols+` FROM playlists WHERE id = ?`, id), &p)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// PlaylistEntries returns a playlist's titles in order (only those present and allowed).
func (s *Store) PlaylistEntries(ctx context.Context, id string, acc Access) ([]PlaylistEntry, []Item, error) {
	q := `SELECT ` + prefixCols("items.", itemCols) + `, e.id FROM playlist_entries e JOIN items ON items.id = e.item_id
		WHERE e.playlist_id = ? AND ` + playableSQL
	args := []any{id}
	q, args = acc.Where("items", q, args)
	rows, err := s.db.QueryContext(ctx, q+` ORDER BY e.position, e.id`, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var entries []PlaylistEntry
	var items []Item
	for rows.Next() {
		var e PlaylistEntry
		var it Item
		if err := scanItemWith(rows, &it, &e.ID); err != nil {
			return nil, nil, err
		}
		e.ItemID = it.ID
		entries = append(entries, e)
		items = append(items, it)
	}
	return entries, items, rows.Err()
}

// CreatePlaylist adds a playlist for a profile.
func (s *Store) CreatePlaylist(ctx context.Context, userID, title string) (*Playlist, error) {
	now := time.Now().Unix()
	p := &Playlist{ID: newID(), UserID: userID, Title: title, CreatedAt: now, UpdatedAt: now}
	_, err := s.db.ExecContext(ctx, `INSERT INTO playlists (id, user_id, title, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		p.ID, p.UserID, p.Title, now, now)
	return p, err
}

// SetPlaylistFilter makes a playlist smart with a filter (JSON), or a list again with "".
func (s *Store) SetPlaylistFilter(ctx context.Context, id, filter string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE playlists SET filter = ?, updated_at = ? WHERE id = ?`, filter, time.Now().Unix(), id)
	return err
}

// RenamePlaylist changes a playlist's title.
func (s *Store) RenamePlaylist(ctx context.Context, id, title string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE playlists SET title = ?, updated_at = ? WHERE id = ?`, title, time.Now().Unix(), id)
	return err
}

// DeletePlaylist removes a playlist (not its titles).
func (s *Store) DeletePlaylist(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM playlists WHERE id = ?`, id)
	return err
}

// AddToPlaylist appends titles to the end of a playlist.
func (s *Store) AddToPlaylist(ctx context.Context, id string, itemIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var next int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), -1) + 1 FROM playlist_entries WHERE playlist_id = ?`, id).Scan(&next); err != nil {
		return err
	}
	for _, it := range itemIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO playlist_entries (playlist_id, item_id, position) VALUES (?, ?, ?)`, id, it, next); err != nil {
			return err
		}
		next++
	}
	if _, err := tx.ExecContext(ctx, `UPDATE playlists SET updated_at = ? WHERE id = ?`, time.Now().Unix(), id); err != nil {
		return err
	}
	return tx.Commit()
}

// RemovePlaylistEntry takes one entry out of a playlist.
func (s *Store) RemovePlaylistEntry(ctx context.Context, id string, entryID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM playlist_entries WHERE playlist_id = ? AND id = ?`, id, entryID)
	if err == nil {
		_, err = s.db.ExecContext(ctx, `UPDATE playlists SET updated_at = ? WHERE id = ?`, time.Now().Unix(), id)
	}
	return err
}

// OrderPlaylist puts a playlist's entries in this order (any left out follow, as they were).
func (s *Store) OrderPlaylist(ctx context.Context, id string, entryIDs []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE playlist_entries SET position = position + 1000000 WHERE playlist_id = ?`, id); err != nil {
		return err
	}
	for i, e := range entryIDs {
		if _, err := tx.ExecContext(ctx, `UPDATE playlist_entries SET position = ? WHERE playlist_id = ? AND id = ?`, i, id, e); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE playlists SET updated_at = ? WHERE id = ?`, time.Now().Unix(), id); err != nil {
		return err
	}
	return tx.Commit()
}
