package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

const (
	clientLogRetention  = 7 * 24 * time.Hour
	clientLogsPerDevice = 4
	clientLogsTotal     = 128
)

type ClientLogEntry struct {
	TimeMS int64  `json:"time_ms"`
	Level  string `json:"level"`
	Event  string `json:"event"`
}

type ClientLogDevice struct {
	ID         int64  `json:"id"`
	Device     string `json:"device"`
	Platform   string `json:"platform"`
	ReceivedAt int64  `json:"received_at"`
}

type ClientLogBatch struct {
	ReceivedAt int64            `json:"received_at"`
	Entries    []ClientLogEntry `json:"entries"`
}

func (s *Store) SaveClientLogs(ctx context.Context, session, platform string, entries []ClientLogEntry, now time.Time) (bool, error) {
	payload, err := json.Marshal(entries)
	if err != nil {
		return false, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	// Check admission in the same write transaction as appending, including concurrent revocation.
	result, err := tx.ExecContext(ctx, `
		INSERT INTO client_logs (session_hash, received_at, platform, entries)
		SELECT token_hash, ?, ?, ? FROM sessions WHERE token_hash = ? AND device <> '' AND is_tool = 0
	`, now.Unix(), platform, string(payload), session)
	if err != nil {
		return false, err
	}
	added, err := result.RowsAffected()
	if err != nil || added == 0 {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `
		DELETE FROM client_logs WHERE received_at <= ?
			OR id IN (SELECT id FROM client_logs WHERE session_hash = ? ORDER BY id DESC LIMIT -1 OFFSET ?)
			OR id IN (SELECT id FROM client_logs ORDER BY id DESC LIMIT -1 OFFSET ?)
	`, now.Add(-clientLogRetention).Unix(), session, clientLogsPerDevice, clientLogsTotal); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func (s *Store) PruneClientLogs(ctx context.Context, now time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, "DELETE FROM client_logs WHERE received_at <= ?", now.Add(-clientLogRetention).Unix())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) ListClientLogDevices(ctx context.Context, now time.Time) ([]ClientLogDevice, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT l.id, s.device, l.platform, l.received_at
		FROM (SELECT MAX(id) AS id FROM client_logs WHERE received_at > ? GROUP BY session_hash) latest
		JOIN client_logs l ON l.id = latest.id JOIN sessions s ON s.token_hash = l.session_hash
		ORDER BY l.id DESC
	`, now.Add(-clientLogRetention).Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanClientLogs(rows, func(d *ClientLogDevice) error {
		return rows.Scan(&d.ID, &d.Device, &d.Platform, &d.ReceivedAt)
	})
}

func (s *Store) ClientLogs(ctx context.Context, id int64, now time.Time) ([]ClientLogBatch, error) {
	cutoff := now.Add(-clientLogRetention).Unix()
	rows, err := s.db.QueryContext(ctx, `
		SELECT received_at, entries FROM client_logs
		WHERE session_hash = (SELECT session_hash FROM client_logs WHERE id = ? AND received_at > ?)
		  AND received_at > ?
		ORDER BY id
	`, id, cutoff, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanClientLogs(rows, func(batch *ClientLogBatch) error {
		var entries string
		if err := rows.Scan(&batch.ReceivedAt, &entries); err != nil {
			return err
		}
		return json.Unmarshal([]byte(entries), &batch.Entries)
	})
}

func scanClientLogs[T any](rows *sql.Rows, scan func(*T) error) ([]T, error) {
	out := []T{}
	for rows.Next() {
		var item T
		if err := scan(&item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
