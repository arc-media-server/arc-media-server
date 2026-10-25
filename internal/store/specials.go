package store

import (
	"context"
	"database/sql"
	"errors"
)

// InlineSpecials follows the profile choice, then the server default, then Off.
func (s *Store) InlineSpecials(ctx context.Context, series, user string) (enabled, overridden bool, err error) {
	var owner string
	err = s.db.QueryRowContext(ctx, `SELECT inline, user_id FROM series_specials
        WHERE series_id = ? AND user_id IN (?, '') ORDER BY user_id = '' LIMIT 1`, series, user).Scan(&enabled, &owner)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, nil
	}
	return enabled, owner != "", err
}

// SetInlineSpecials changes a personal choice; admins also set the inherited default.
// nil clears that scope. Both writes are atomic and independent of display order.
func (s *Store) SetInlineSpecials(ctx context.Context, series, user string, admin bool, enabled *bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	owners := []string{user}
	if admin {
		owners = append(owners, "")
	}
	for _, owner := range owners {
		if enabled == nil {
			_, err = tx.ExecContext(ctx, `DELETE FROM series_specials WHERE series_id = ? AND user_id = ?`, series, owner)
		} else {
			_, err = tx.ExecContext(ctx, `INSERT INTO series_specials(series_id,user_id,inline) VALUES(?,?,?)
                ON CONFLICT(series_id,user_id) DO UPDATE SET inline=excluded.inline`, series, owner, boolInt(*enabled))
		}
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ReplaceSpecialPlacements accepts only two distinct episode identities in this series.
func (s *Store) ReplaceSpecialPlacements(ctx context.Context, series string, before map[string]string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM episode_specials WHERE episode_id IN (SELECT id FROM items WHERE parent_id=?)`, series); err != nil {
		return err
	}
	for ep, target := range before {
		if ep == target {
			continue
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO episode_specials(episode_id,before_episode_id)
            SELECT a.id,b.id FROM items a JOIN items b ON b.id=?
            WHERE a.id=? AND a.parent_id=? AND b.parent_id=? AND a.kind='episode' AND b.kind='episode'`, target, ep, series, series); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DisplayEntries is the shared browse/playback traversal. Special labels remain S00.
// Manual entries and custom orders win; absent/unmapped hints stay in Specials.
func (s *Store) DisplayEntries(ctx context.Context, orderingID int64, user string) ([]OrderingEntry, error) {
	entries, err := s.OrderingEntries(ctx, orderingID)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		entries[i].GroupSeason = entries[i].Season
	}
	o, err := s.GetOrdering(ctx, orderingID)
	if err != nil || o == nil {
		return entries, err
	}
	inline, _, err := s.InlineSpecials(ctx, o.SeriesID, user)
	if err != nil {
		return nil, err
	}
	before := map[string]string{}
	if inline && o.Source != "custom" {
		rows, err := s.db.QueryContext(ctx, `SELECT p.episode_id,p.before_episode_id FROM episode_specials p
            JOIN items i ON i.id=p.episode_id WHERE i.parent_id=?`, o.SeriesID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var ep, target string
			if err := rows.Scan(&ep, &target); err != nil {
				rows.Close()
				return nil, err
			}
			before[ep] = target
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	// First position wins, matching the existing season response's identity de-duplication.
	unique := []OrderingEntry{}
	seen := map[string]bool{}
	for _, e := range entries {
		if !seen[e.EpisodeID] {
			unique = append(unique, e)
			seen[e.EpisodeID] = true
		}
	}
	targets := map[string]OrderingEntry{}
	for _, e := range unique {
		if e.Season > 0 {
			targets[e.EpisodeID] = e
		}
	}
	insert := map[string][]OrderingEntry{}
	moved := map[string]bool{}
	for _, e := range unique {
		target, ok := targets[before[e.EpisodeID]]
		if e.Season == 0 && !e.Locked && ok {
			e.GroupSeason, e.InlineSpecial = target.Season, true
			insert[target.EpisodeID] = append(insert[target.EpisodeID], e)
			moved[e.EpisodeID] = true
		}
	}
	out := []OrderingEntry{}
	specials := []OrderingEntry{}
	inserted := map[string]bool{}
	for _, e := range entries {
		if moved[e.EpisodeID] {
			continue
		}
		if e.Season == 0 {
			specials = append(specials, e)
			continue
		}
		if !inserted[e.EpisodeID] {
			out = append(out, insert[e.EpisodeID]...)
			inserted[e.EpisodeID] = true
		}
		out = append(out, e)
	}
	return append(out, specials...), nil
}
