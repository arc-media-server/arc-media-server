package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrInvalidImportProfiles = errors.New("invalid import profile mapping")

// ImportProfileTarget has already been matched to a known source account by the API.
type ImportProfileTarget struct{ AccountID, Name, Target string }

// PrepareImportProfiles validates all targets and creates requested profiles in one transaction.
// A bad later mapping or a colliding generated name cannot leave earlier profiles behind.
func (s *Store) PrepareImportProfiles(ctx context.Context, targets []ImportProfileTarget, source string) (map[string]string, map[string]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id,name FROM users`)
	if err != nil {
		return nil, nil, err
	}
	names := map[string]string{}
	for rows.Next() {
		var id, name string
		if err = rows.Scan(&id, &name); err != nil {
			rows.Close()
			return nil, nil, err
		}
		names[id] = name
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, err
	}
	chosen := 0
	seen := map[string]bool{}
	for _, a := range targets {
		if a.AccountID == "" || seen[a.AccountID] {
			return nil, nil, ErrInvalidImportProfiles
		}
		seen[a.AccountID] = true
		if a.Target == "" {
			continue
		}
		chosen++
		if a.Target == "new" {
			if strings.TrimSpace(a.Name) == "" {
				return nil, nil, fmt.Errorf("%w: imported profile has no name", ErrInvalidImportProfiles)
			}
		} else if _, ok := names[a.Target]; !ok {
			return nil, nil, fmt.Errorf("%w: choose an existing profile for every selected person", ErrInvalidImportProfiles)
		}
	}
	if chosen == 0 {
		return nil, nil, fmt.Errorf("%w: choose a profile for at least one person", ErrInvalidImportProfiles)
	}
	mapping := map[string]string{}
	for _, a := range targets {
		if a.Target == "" {
			continue
		}
		target := a.Target
		if target == "new" {
			id, name := NewID(), a.Name
			insert := func() error {
				_, e := tx.ExecContext(ctx, `INSERT INTO users(id,name,password_hash,is_admin,created_at,pin_hash,color) VALUES (?,?,'',0,?,'','slate')`, id, name, time.Now().Unix())
				return e
			}
			err = insert()
			if isUniqueViolation(err) {
				name = a.Name + " (" + source + ")"
				err = insert()
			}
			if isUniqueViolation(err) {
				return nil, nil, ErrNameTaken
			}
			if err != nil {
				return nil, nil, err
			}
			target, names[id] = id, name
		}
		mapping[a.AccountID] = target
	}
	if err = tx.Commit(); err != nil {
		return nil, nil, err
	}
	return mapping, names, nil
}
