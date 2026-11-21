package store

import (
	"context"
	"errors"
	"time"
)

// CreateToolSession creates an ordinary password-authenticated device session with the
// creating administrator selected. is_tool is display metadata, never an authorization bypass.
func (s *Store) CreateToolSession(ctx context.Context, hash, userID, name string) error {
	now := time.Now().Unix()
	result, err := s.db.ExecContext(ctx, `INSERT INTO sessions (token_hash, user_id, via, device, created_at, last_seen_at, is_tool)
        SELECT ?, id, 'password', ?, ?, ?, 1 FROM users WHERE id = ? AND is_admin = 1`, hash, name, now, now, userID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("tool token requires an administrator")
	}
	return nil
}
