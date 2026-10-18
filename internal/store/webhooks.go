package store

import (
	"context"
	"encoding/json"
	"slices"
	"time"
)

// Webhook is an address Arc posts events to.
type Webhook struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
	// The events it gets (e.g. "library.new"); empty for all of them.
	Events    []string `json:"events"`
	CreatedAt int64    `json:"created_at"`
}

// Wants reports whether the webhook gets an event.
func (h Webhook) Wants(event string) bool {
	return len(h.Events) == 0 || slices.Contains(h.Events, event)
}

// Webhooks lists every webhook, oldest first.
func (s *Store) Webhooks(ctx context.Context) ([]Webhook, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, url, events, created_at FROM webhooks ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Webhook{}
	for rows.Next() {
		var h Webhook
		var events string
		if err := rows.Scan(&h.ID, &h.Name, &h.URL, &events, &h.CreatedAt); err != nil {
			return nil, err
		}
		h.Events = []string{}
		json.Unmarshal([]byte(events), &h.Events)
		out = append(out, h)
	}
	return out, rows.Err()
}

// SaveWebhook adds a webhook (no id) or changes one.
func (s *Store) SaveWebhook(ctx context.Context, h *Webhook) error {
	if h.Events == nil {
		h.Events = []string{}
	}
	events, _ := json.Marshal(h.Events)
	if h.ID == "" {
		h.ID, h.CreatedAt = newID(), time.Now().Unix()
		_, err := s.db.ExecContext(ctx, `INSERT INTO webhooks (id, name, url, events, created_at) VALUES (?, ?, ?, ?, ?)`,
			h.ID, h.Name, h.URL, string(events), h.CreatedAt)
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE webhooks SET name = ?, url = ?, events = ? WHERE id = ?`, h.Name, h.URL, string(events), h.ID)
	return err
}

// DeleteWebhook removes a webhook.
func (s *Store) DeleteWebhook(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM webhooks WHERE id = ?`, id)
	return err
}

// AddedSince lists movies and episodes in a library first seen at or after a time (unix
// seconds) that have a file, newest first, up to a limit.
func (s *Store) AddedSince(ctx context.Context, libraryID, since int64, limit int) ([]Item, error) {
	return s.queryItems(ctx, `SELECT `+itemCols+` FROM items WHERE library_id = ? AND added_at >= ?
		AND kind IN ('movie', 'episode') AND `+playableSQL+` ORDER BY added_at DESC LIMIT ?`, libraryID, since, limit)
}

// NewsEntry is something new for a profile: a movie, or a show with new episodes.
type NewsEntry struct {
	Item    Item
	Count   int // a show's new episodes
	AddedAt int64
}

// News lists what's arrived since a time that a profile would want to know about: shows it
// watches with new episodes it hasn't seen, and new movies it hasn't watched, newest first. Titles
// from a library's first scan don't count, so adding a library isn't news.
func (s *Store) News(ctx context.Context, userID string, acc Access, since int64, limit int) ([]NewsEntry, error) {
	after := `items.added_at >= ? AND items.added_at > (SELECT created_at + 3600 FROM libraries l WHERE l.id = items.library_id)`
	q := `SELECT ` + itemCols + `, n, last FROM (
			SELECT parent_id AS sid, COUNT(*) AS n, MAX(added_at) AS last FROM items
			WHERE kind = 'episode' AND ` + after + ` AND ` + playableSQL + `
				AND parent_id IN (SELECT DISTINCT i.parent_id FROM user_item_data u JOIN items i ON i.id = u.item_id
					WHERE u.user_id = ? AND i.kind = 'episode' AND (u.played = 1 OR u.position_ms > 0))
				AND NOT EXISTS (SELECT 1 FROM user_item_data u WHERE u.user_id = ? AND u.item_id = items.id AND u.played = 1)
			GROUP BY parent_id
		) JOIN items ON items.id = sid WHERE 1 = 1`
	q, args := acc.Where("items", q, []any{since, userID, userID})
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	var out []NewsEntry
	for rows.Next() {
		var e NewsEntry
		if err := scanItemWith(rows, &e.Item, &e.Count, &e.AddedAt); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	mq := `SELECT ` + itemCols + `, added_at FROM items WHERE kind = 'movie' AND ` + after + ` AND ` + playableSQL + `
		AND NOT EXISTS (SELECT 1 FROM user_item_data u WHERE u.user_id = ? AND u.item_id = items.id AND u.played = 1)`
	mq, args = acc.Where("items", mq, []any{since, userID})
	rows, err = s.db.QueryContext(ctx, mq+` ORDER BY added_at DESC LIMIT ?`, append(args, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		e := NewsEntry{Count: 1}
		if err := scanItemWith(rows, &e.Item, &e.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	slices.SortStableFunc(out, func(a, b NewsEntry) int { return int(b.AddedAt - a.AddedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// NewsSeen is when a profile last looked at what's new (unix seconds).
func (s *Store) NewsSeen(ctx context.Context, userID string) (int64, error) {
	var t int64
	err := s.db.QueryRowContext(ctx, `SELECT news_seen_at FROM users WHERE id = ?`, userID).Scan(&t)
	return t, err
}

// SetNewsSeen records that a profile has looked at what's new.
func (s *Store) SetNewsSeen(ctx context.Context, userID string, at int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET news_seen_at = ? WHERE id = ?`, at, userID)
	return err
}
