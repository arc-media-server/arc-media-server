package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

// User is a profile. The admin profile holds the household password; others may have a PIN.
type User struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	IsAdmin bool   `json:"is_admin"`
	Color   string `json:"color"`
	HasPIN  bool   `json:"has_pin"`
	// The oldest age rating the profile may watch; 0 means no limit.
	MaxAge int `json:"max_age"`
	// The libraries the profile may see; nil for all of them.
	LibraryIDs []int64 `json:"library_ids"`
	// Titles with these labels are hidden from the profile.
	ExcludeLabels []string `json:"exclude_labels"`
	// A friend: someone outside the household who signs in with their own name and password.
	IsFriend     bool   `json:"is_friend"`
	PasswordHash string `json:"-"`
	PINHash      string `json:"-"`
	CreatedAt    int64  `json:"created_at"`
}

const userCols = `id, name, is_admin, color, pin_hash <> '', password_hash, pin_hash, created_at, max_age, library_ids, exclude_labels, is_friend`

func scanUser(row interface{ Scan(...any) error }, u *User) error {
	var libs, labels string
	if err := row.Scan(&u.ID, &u.Name, &u.IsAdmin, &u.Color, &u.HasPIN, &u.PasswordHash, &u.PINHash, &u.CreatedAt, &u.MaxAge, &libs, &labels, &u.IsFriend); err != nil {
		return err
	}
	u.LibraryIDs = parseIDs(libs)
	u.ExcludeLabels = []string{}
	json.Unmarshal([]byte(labels), &u.ExcludeLabels)
	return nil
}

// parseIDs reads a comma-separated list of library ids; "" means all libraries (nil).
func parseIDs(s string) []int64 {
	if s == "" {
		return nil
	}
	out := []int64{}
	for _, part := range strings.Split(s, ",") {
		if id, err := strconv.ParseInt(part, 10, 64); err == nil && id > 0 {
			out = append(out, id)
		}
	}
	return out
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
	MaxAge       *int
	// The libraries the profile may see: set with a list (nil field leaves it alone); AllLibraries
	// lifts the restriction.
	LibraryIDs    *[]int64
	AllLibraries  bool
	ExcludeLabels *[]string
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
	if up.MaxAge != nil {
		sets = append(sets, "max_age = ?")
		args = append(args, *up.MaxAge)
	}
	if up.AllLibraries {
		sets = append(sets, "library_ids = ''")
	} else if up.LibraryIDs != nil {
		ids := make([]string, 0, len(*up.LibraryIDs))
		for _, id := range *up.LibraryIDs {
			ids = append(ids, strconv.FormatInt(id, 10))
		}
		// An empty list (no libraries) is kept as "0", since '' means all of them.
		list := strings.Join(ids, ",")
		if list == "" {
			list = "0"
		}
		sets = append(sets, "library_ids = ?")
		args = append(args, list)
	}
	if up.ExcludeLabels != nil {
		b, _ := json.Marshal(*up.ExcludeLabels)
		sets = append(sets, "exclude_labels = ?")
		args = append(args, string(b))
	}
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
	return err != nil && (strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "SQLSTATE 23505"))
}

// Session is a signed-in browser.
type Session struct {
	TokenHash string
	UserID    string // "" until a profile is picked
	Via       string // password, local or friend
	CreatedAt int64
	LastSeen  int64
	// An app's name for itself; "" for a browser.
	Device string
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
		SELECT token_hash, COALESCE(user_id, ''), via, created_at, last_seen_at, device FROM sessions WHERE token_hash = ?`,
		tokenHash).Scan(&se.TokenHash, &se.UserID, &se.Via, &se.CreatedAt, &se.LastSeen, &se.Device)
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

// UserPrefs returns a profile's preferences JSON ("{}" when none are set).
func (s *Store) UserPrefs(ctx context.Context, userID string) (string, error) {
	var p string
	err := s.db.QueryRowContext(ctx, `SELECT prefs FROM users WHERE id = ?`, userID).Scan(&p)
	if errors.Is(err, sql.ErrNoRows) {
		return "{}", nil
	}
	return p, err
}

// SetUserPrefs replaces a profile's preferences JSON.
func (s *Store) SetUserPrefs(ctx context.Context, userID, prefs string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET prefs = ? WHERE id = ?`, prefs, userID)
	return err
}

// SetInvite makes a profile a friend with an invite link (its token's hash) that works until a time.
func (s *Store) SetInvite(ctx context.Context, userID, tokenHash string, expires int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET is_friend = 1, invite_hash = ?, invite_expires = ? WHERE id = ?`,
		tokenHash, expires, userID)
	return err
}

// UserByInvite finds the friend an invite link is for, or nil when it's unknown or has run out.
func (s *Store) UserByInvite(ctx context.Context, tokenHash string) (*User, error) {
	var u User
	err := scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users
		WHERE invite_hash = ? AND invite_hash <> '' AND invite_expires > ? AND is_friend = 1`, tokenHash, time.Now().Unix()), &u)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// AcceptInvite sets a friend's password and uses up their invite link.
func (s *Store) AcceptInvite(ctx context.Context, userID, passwordHash string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash = ?, invite_hash = '', invite_expires = 0 WHERE id = ?`,
		passwordHash, userID)
	return err
}

// FriendByName finds a friend by name (any case), or nil.
func (s *Store) FriendByName(ctx context.Context, name string) (*User, error) {
	var u User
	err := scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE name = ? COLLATE NOCASE AND is_friend = 1`, name), &u)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// DeleteUserSessions signs a profile out everywhere.
func (s *Store) DeleteUserSessions(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

// SetSessionDevice names the app a session belongs to.
func (s *Store) SetSessionDevice(ctx context.Context, tokenHash, device string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET device = ? WHERE token_hash = ?`, device, tokenHash)
	return err
}

// SessionInfo is a signed-in browser or app, for Settings > Devices.
type SessionInfo struct {
	TokenHash string `json:"id"`
	Device    string `json:"device,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
	Via       string `json:"via"`
	UserID    string `json:"profile_id,omitempty"`
	UserName  string `json:"profile,omitempty"`
	CreatedAt int64  `json:"created_at"`
	LastSeen  int64  `json:"last_seen_at"`
}

// ListSessions lists every signed-in browser and app, most recently used first.
func (s *Store) ListSessions(ctx context.Context) ([]SessionInfo, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT se.token_hash, se.device, se.user_agent, se.via, COALESCE(se.user_id, ''), COALESCE(u.name, ''), se.created_at, se.last_seen_at
		FROM sessions se LEFT JOIN users u ON u.id = se.user_id
		ORDER BY se.last_seen_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SessionInfo
	for rows.Next() {
		var si SessionInfo
		if err := rows.Scan(&si.TokenHash, &si.Device, &si.UserAgent, &si.Via, &si.UserID, &si.UserName, &si.CreatedAt, &si.LastSeen); err != nil {
			return nil, err
		}
		out = append(out, si)
	}
	return out, rows.Err()
}
