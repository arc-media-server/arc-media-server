package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"math/big"
	"slices"
	"time"
)

const MaxQueueEntries = 1000
const QueueLifetime = 7 * 24 * time.Hour

var ErrQueueNotFound = errors.New("queue not found")
var ErrQueueConflict = errors.New("queue changed; reload it and retry")
var ErrQueueInput = errors.New("invalid queue operation or entries")

type PlayQueueEntry struct {
	ID               string `json:"id"`
	ItemID           string `json:"item_id"`
	OriginalPosition int    `json:"-"`
}
type PlayQueue struct {
	ID             string           `json:"id"`
	Revision       int64            `json:"revision"`
	CurrentEntryID string           `json:"current_entry_id"`
	Shuffled       bool             `json:"shuffled"`
	ExpiresAt      int64            `json:"expires_at"`
	Entries        []PlayQueueEntry `json:"entries"`
}
type queueReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

// One joined statement gives metadata and entries the same SQLite read snapshot. A pair of
// independent reads could otherwise combine an old revision with a concurrently edited list.
func loadPlayQueue(ctx context.Context, db queueReader, user, session, id string) (*PlayQueue, error) {
	query := `SELECT q.id,q.revision,q.current_entry_id,q.shuffled,q.expires_at,e.id,e.item_id,e.original_position
 FROM play_queues q LEFT JOIN play_queue_entries e ON e.queue_id=q.id
 WHERE q.user_id=? AND q.session_hash=? AND q.expires_at>?`
	args := []any{user, session, time.Now().Unix()}
	if id != "" {
		query += " AND q.id=?"
		args = append(args, id)
	}
	query += " ORDER BY e.position,e.id"
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	q := &PlayQueue{Entries: []PlayQueueEntry{}}
	found, currentExists := false, false
	for rows.Next() {
		var entryID, itemID sql.NullString
		var original sql.NullInt64
		if err = rows.Scan(&q.ID, &q.Revision, &q.CurrentEntryID, &q.Shuffled, &q.ExpiresAt, &entryID, &itemID, &original); err != nil {
			return nil, err
		}
		found = true
		if entryID.Valid {
			q.Entries = append(q.Entries, PlayQueueEntry{ID: entryID.String, ItemID: itemID.String, OriginalPosition: int(original.Int64)})
			currentExists = currentExists || entryID.String == q.CurrentEntryID
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrQueueNotFound
	}
	if !currentExists {
		q.CurrentEntryID = ""
	}
	return q, nil
}
func (s *Store) PlayQueue(ctx context.Context, user, session, id string) (*PlayQueue, error) {
	q, err := loadPlayQueue(ctx, s.db, user, session, id)
	if err != nil {
		return nil, err
	}
	// A bounded lease extension (at most hourly) lets active queues survive without writing on
	// every UI poll. Revocation still deletes queues through the session foreign key.
	expiry := time.Now().Add(QueueLifetime).Unix()
	if expiry-q.ExpiresAt >= 3600 {
		if _, err = s.db.ExecContext(ctx, "UPDATE play_queues SET expires_at=? WHERE id=? AND user_id=? AND session_hash=? AND expires_at>?", expiry, q.ID, user, session, time.Now().Unix()); err != nil {
			return nil, err
		}
		q.ExpiresAt = expiry
	}
	return q, nil
}
func (s *Store) CreatePlayQueue(ctx context.Context, user, session string, items []string, replaceID string, revision int64) (*PlayQueue, error) {
	if user == "" || session == "" || len(items) == 0 || len(items) > MaxQueueEntries {
		return nil, ErrQueueInput
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "DELETE FROM play_queues WHERE expires_at<=?", time.Now().Unix()); err != nil {
		return nil, err
	}
	existing, err := loadPlayQueue(ctx, tx, user, session, "")
	if err != nil && !errors.Is(err, ErrQueueNotFound) {
		return nil, err
	}
	if existing != nil {
		if existing.ID != replaceID || existing.Revision != revision {
			return nil, ErrQueueConflict
		}
		if _, err = tx.ExecContext(ctx, "DELETE FROM play_queues WHERE id=?", existing.ID); err != nil {
			return nil, err
		}
	} else if replaceID != "" || revision != 0 {
		return nil, ErrQueueConflict
	}
	q := &PlayQueue{ID: newID(), Revision: 1, ExpiresAt: time.Now().Add(QueueLifetime).Unix(), Entries: []PlayQueueEntry{}}
	for i, item := range items {
		q.Entries = append(q.Entries, PlayQueueEntry{ID: newID(), ItemID: item, OriginalPosition: i})
	}
	q.CurrentEntryID = q.Entries[0].ID
	if _, err = tx.ExecContext(ctx, `INSERT INTO play_queues(id,user_id,session_hash,revision,current_entry_id,expires_at) VALUES(?,?,?,?,?,?)`, q.ID, user, session, q.Revision, q.CurrentEntryID, q.ExpiresAt); err != nil {
		return nil, err
	}
	if err = saveQueueEntries(ctx, tx, q); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return q, nil
}

type QueueMutation struct {
	Revision  int64    `json:"revision"`
	Operation string   `json:"operation"`
	ItemIDs   []string `json:"item_ids" api:"optional"`
	EntryIDs  []string `json:"entry_ids" api:"optional"`
	EntryID   string   `json:"entry_id" api:"optional"`
}

func (s *Store) MutatePlayQueue(ctx context.Context, user, session, id string, m QueueMutation) (*PlayQueue, error) {
	if len(m.ItemIDs) > MaxQueueEntries || len(m.EntryIDs) > MaxQueueEntries {
		return nil, ErrQueueInput
	}
	switch m.Operation {
	case "append", "next":
		if len(m.EntryIDs) > 0 || m.EntryID != "" {
			return nil, ErrQueueInput
		}
	case "remove", "reorder":
		if len(m.ItemIDs) > 0 || m.EntryID != "" {
			return nil, ErrQueueInput
		}
	case "select":
		if len(m.ItemIDs) > 0 || len(m.EntryIDs) > 0 {
			return nil, ErrQueueInput
		}
	default:
		if len(m.ItemIDs) > 0 || len(m.EntryIDs) > 0 || m.EntryID != "" {
			return nil, ErrQueueInput
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	q, err := loadPlayQueue(ctx, tx, user, session, id)
	if err != nil {
		return nil, err
	}
	if m.Revision != q.Revision {
		return nil, ErrQueueConflict
	}
	current := slices.IndexFunc(q.Entries, func(e PlayQueueEntry) bool { return e.ID == q.CurrentEntryID })
	if current < 0 {
		current = len(q.Entries) - 1
	}
	switch m.Operation {
	case "append", "next":
		if len(m.ItemIDs) == 0 || len(q.Entries)+len(m.ItemIDs) > MaxQueueEntries {
			return nil, ErrQueueInput
		}
		maxOriginal := -1
		for _, e := range q.Entries {
			maxOriginal = max(maxOriginal, e.OriginalPosition)
		}
		added := make([]PlayQueueEntry, 0, len(m.ItemIDs))
		for i, item := range m.ItemIDs {
			added = append(added, PlayQueueEntry{ID: newID(), ItemID: item, OriginalPosition: maxOriginal + 1 + i})
		}
		at := len(q.Entries)
		if m.Operation == "next" {
			at = current + 1
		}
		q.Entries = slices.Insert(q.Entries, at, added...)
		if q.CurrentEntryID == "" {
			q.CurrentEntryID = added[0].ID
		}
	case "remove":
		if len(m.EntryIDs) == 0 {
			return nil, ErrQueueInput
		}
		wanted := map[string]bool{}
		for _, id := range m.EntryIDs {
			if wanted[id] || id == q.CurrentEntryID {
				return nil, ErrQueueInput
			}
			wanted[id] = true
		}
		for _, e := range q.Entries {
			delete(wanted, e.ID)
		}
		if len(wanted) > 0 {
			return nil, ErrQueueInput
		}
		q.Entries = slices.DeleteFunc(q.Entries, func(e PlayQueueEntry) bool { return slices.Contains(m.EntryIDs, e.ID) })
	case "reorder":
		if len(m.EntryIDs) != len(q.Entries) {
			return nil, ErrQueueInput
		}
		reordered := make([]PlayQueueEntry, 0, len(q.Entries))
		byID := map[string]PlayQueueEntry{}
		for _, e := range q.Entries {
			byID[e.ID] = e
		}
		for i, id := range m.EntryIDs {
			e, ok := byID[id]
			if !ok {
				return nil, ErrQueueInput
			}
			if i <= current && q.Entries[i].ID != id {
				return nil, ErrQueueInput
			}
			delete(byID, id)
			e.OriginalPosition = i
			reordered = append(reordered, e)
		}
		q.Entries = reordered
		q.Shuffled = false
	case "clear":
		q.Entries = q.Entries[:current+1]
	case "advance":
		q.CurrentEntryID = ""
		if current+1 < len(q.Entries) {
			q.CurrentEntryID = q.Entries[current+1].ID
		}
	case "select":
		if !slices.ContainsFunc(q.Entries, func(e PlayQueueEntry) bool { return e.ID == m.EntryID }) {
			return nil, ErrQueueInput
		}
		q.CurrentEntryID = m.EntryID
	case "shuffle":
		if !q.Shuffled {
			for i := len(q.Entries) - 1; i > current+1; i-- {
				n, e := rand.Int(rand.Reader, big.NewInt(int64(i-current)))
				if e != nil {
					return nil, e
				}
				j := current + 1 + int(n.Int64())
				q.Entries[i], q.Entries[j] = q.Entries[j], q.Entries[i]
			}
			q.Shuffled = true
		}
	case "unshuffle":
		slices.SortStableFunc(q.Entries[current+1:], func(a, b PlayQueueEntry) int { return a.OriginalPosition - b.OriginalPosition })
		q.Shuffled = false
	default:
		return nil, ErrQueueInput
	}
	if !q.Shuffled {
		for i := range q.Entries {
			q.Entries[i].OriginalPosition = i
		}
	}
	q.Revision++
	q.ExpiresAt = time.Now().Add(QueueLifetime).Unix()
	if _, err = tx.ExecContext(ctx, "UPDATE play_queues SET revision=?,current_entry_id=?,shuffled=?,expires_at=? WHERE id=?", q.Revision, q.CurrentEntryID, q.Shuffled, q.ExpiresAt, q.ID); err != nil {
		return nil, err
	}
	if err = saveQueueEntries(ctx, tx, q); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return q, nil
}
func saveQueueEntries(ctx context.Context, tx *sql.Tx, q *PlayQueue) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM play_queue_entries WHERE queue_id=?", q.ID); err != nil {
		return err
	}
	for i, e := range q.Entries {
		if _, err := tx.ExecContext(ctx, "INSERT INTO play_queue_entries(id,queue_id,item_id,position,original_position) VALUES(?,?,?,?,?)", e.ID, q.ID, e.ItemID, i, e.OriginalPosition); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) DeletePlayQueue(ctx context.Context, user, session, id string, revision int64) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM play_queues WHERE id=? AND user_id=? AND session_hash=? AND revision=? AND expires_at>?", id, user, session, revision, time.Now().Unix())
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		if _, err = loadPlayQueue(ctx, s.db, user, session, id); err != nil {
			return err
		}
		return ErrQueueConflict
	}
	return nil
}

// QueueItems returns accessible, present movies/episodes in bounded chunks. Entries themselves
// can remain as opaque placeholders after permissions or availability change.
func (s *Store) QueueItems(ctx context.Context, ids []string, acc Access) (map[string]Item, error) {
	out := map[string]Item{}
	for chunk := range chunks(ids, 500) {
		args := make([]any, len(chunk))
		for i, id := range chunk {
			args[i] = id
		}
		query := `SELECT ` + itemCols + ` FROM items WHERE id IN (` + placeholders(len(chunk)) + `) AND kind IN ('movie','episode') AND EXISTS(SELECT 1 FROM item_files f JOIN media_files m ON m.id=f.file_id WHERE f.item_id=items.id AND m.missing_since IS NULL)`
		query, args = acc.Where("items", query, args)
		items, err := s.queryItems(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			out[it.ID] = it
		}
	}
	return out, nil
}
