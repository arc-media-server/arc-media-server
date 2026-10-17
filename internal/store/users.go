package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// User is a profile. The admin profile holds the household password; others may have a PIN.
type User struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	IsAdmin      bool   `json:"is_admin"`
	Color        string `json:"color"`
	HasPIN       bool   `json:"has_pin"`
	PasswordHash string `json:"-"`
	PINHash      string `json:"-"`
	CreatedAt    int64  `json:"created_at"`
}

const userCols = `id, name, is_admin, color, pin_hash <> '', password_hash, pin_hash, created_at`

func scanUser(row interface{ Scan(...any) error }, u *User) error {
	return row.Scan(&u.ID, &u.Name, &u.IsAdmin, &u.Color, &u.HasPIN, &u.PasswordHash, &u.PINHash, &u.CreatedAt)
}

// ErrNameTaken means another profile already has the name.
var ErrNameTaken = errors.New("a profile with that name already exists")

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// CreateUser adds a profile and returns it.
func (s *Store) CreateUser(ctx context.Context, name string, isAdmin bool, color, passwordHash, pinHash string) (*User, error) {
	u := &User{ID: NewID(), Name: name, IsAdmin: isAdmin, Color: color, HasPIN: pinHash != "",
		PasswordHash: passwordHash, PINHash: pinHash, CreatedAt: time.Now().Unix()}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (id, name, password_hash, is_admin, created_at, pin_hash, color)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, u.ID, u.Name, passwordHash, boolInt(isAdmin), u.CreatedAt, pinHash, color)
	if isUniqueViolation(err) {
		return nil, ErrNameTaken
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

// GetUser returns nil when there's no such profile.
func (s *Store) GetUser(ctx context.Context, id string) (*User, error) {
	var u User
	err := scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = ?`, id), &u)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// AdminUser returns the household admin, or nil before setup.
func (s *Store) AdminUser(ctx context.Context) (*User, error) {
	var u User
	err := scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE is_admin = 1 ORDER BY created_at LIMIT 1`), &u)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// ListUsers returns the profiles, admin first, then in the order they were added.
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+userCols+` FROM users ORDER BY is_admin DESC, created_at, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if err := scanUser(rows, &u); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// UserUpdate changes a profile. Nil fields are left alone; an empty PINHash removes the PIN.
type UserUpdate struct {
	Name         *string
	Color        *string
	PINHash      *string
	PasswordHash *string
}

func (s *Store) UpdateUser(ctx context.Context, id string, up UserUpdate) error {
	sets, args := []string{}, []any{}
	add := func(col string, v *string) {
		if v != nil {
			sets = append(sets, col+" = ?")
			args = append(args, *v)
		}
	}
	add("name", up.Name)
	add("color", up.Color)
	add("pin_hash", up.PINHash)
	add("password_hash", up.PasswordHash)
	if len(sets) == 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `UPDATE users SET `+strings.Join(sets, ", ")+` WHERE id = ?`, append(args, id)...)
	if isUniqueViolation(err) {
		return ErrNameTaken
	}
	return err
}

// DeleteUser removes a profile with its watch history and sessions.
func (s *Store) DeleteUser(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ? AND is_admin = 0`, id)
	return err
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// Session is a signed-in browser.
type Session struct {
	TokenHash string
	UserID    string // "" until a profile is picked
	Via       string // password or local
	CreatedAt int64
	LastSeen  int64
}

func (s *Store) CreateSession(ctx context.Context, tokenHash, userID, via, userAgent string) error {
	now := time.Now().Unix()
	var uid any
	if userID != "" {
		uid = userID
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (token_hash, user_id, via, user_agent, created_at, last_seen_at) VALUES (?, ?, ?, ?, ?, ?)`,
		tokenHash, uid, via, userAgent, now, now)
	return err
}

// GetSession returns nil for an unknown token.
func (s *Store) GetSession(ctx context.Context, tokenHash string) (*Session, error) {
	var se Session
	err := s.db.QueryRowContext(ctx, `
		SELECT token_hash, COALESCE(user_id, ''), via, created_at, last_seen_at FROM sessions WHERE token_hash = ?`,
		tokenHash).Scan(&se.TokenHash, &se.UserID, &se.Via, &se.CreatedAt, &se.LastSeen)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &se, nil
}

// SetSessionUser sets (or with "" clears) the profile a session uses.
func (s *Store) SetSessionUser(ctx context.Context, tokenHash, userID string) error {
	var uid any
	if userID != "" {
		uid = userID
	}
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET user_id = ? WHERE token_hash = ?`, uid, tokenHash)
	return err
}

// TouchSession records that a session was used.
func (s *Store) TouchSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET last_seen_at = ? WHERE token_hash = ?`, time.Now().Unix(), tokenHash)
	return err
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

// DeletePasswordSessions signs out every browser that signed in with the password (e.g. after it
// changes), except the given one.
func (s *Store) DeletePasswordSessions(ctx context.Context, exceptTokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE via = 'password' AND token_hash <> ?`, exceptTokenHash)
	return err
}

// DeleteStaleSessions removes sessions unused since before.
func (s *Store) DeleteStaleSessions(ctx context.Context, before int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE last_seen_at < ?`, before)
	return err
}
