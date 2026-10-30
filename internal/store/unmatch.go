package store

import (
	"context"
	"errors"
	"strings"
	"time"
)

// UnmatchEpisode carries filename-derived labels/positions for existing identities, never new IDs.
type UnmatchEpisode struct {
	ID, Title, Key  string
	Season, Episode int
	Placed          bool
}

// Unmatch clears title enrichment atomically. It deliberately does not touch files, probe data,
// watch state, playlists, manual collections, arrivals, source numbering or instance settings.
// Call under the scanner library lease so no metadata worker can repopulate a half-reset show.
func (s *Store) Unmatch(ctx context.Context, id, title string, year int, episodes []UnmatchEpisode) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var kind string
	if err = tx.QueryRowContext(ctx, "SELECT kind FROM items WHERE id=?", id).Scan(&kind); err != nil {
		return err
	}
	if kind != "series" {
		return errors.New("only shows can be unmatched")
	}
	rows, err := tx.QueryContext(ctx, "SELECT id FROM items WHERE id=? OR parent_id=?", id, id)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var key string
		if err = rows.Scan(&key); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, key)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	members := map[string]bool{}
	for _, key := range ids {
		members[key] = true
	}
	for _, ep := range episodes {
		if ep.ID == id || !members[ep.ID] {
			return errors.New("episode is not in this show")
		}
	}
	// Reset only enrichment columns, not file/runtime/identity/user records. Source numbering is
	// configuration describing filenames and is retained. Display orders and overrides are reset.
	reset := `UPDATE items SET original_title='',overview='',release_date='',tmdb_id=0,tvdb_id=0,imdb_id='',
 poster_path='',backdrop_path='',logo_path='',local_poster='',local_backdrop='',local_logo='',local_theme='',
 locked_fields='',tagline='',content_rating='',studio='',rating=0,original_language='',show_status='',
 last_aired='',next_aired='',imdb_rating=0,imdb_votes=0,rt_critics=NULL,metacritic=NULL,
 tmdb_collection_id=0,details_provider='',match_info='',synced_at=0,min_age=-1,runtime_min=0,
 metadata_state='unmatched',match_suppressed=1,updated_at=? WHERE id=?`
	for _, key := range ids {
		if _, err = tx.ExecContext(ctx, reset, time.Now().Unix(), key); err != nil {
			return err
		}
		for _, table := range []string{"item_genres", "item_people", "item_alt_titles", "item_keywords", "item_recommendations"} {
			if _, err = tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE item_id=?", key); err != nil {
				return err
			}
		}
		if key != id {
			if _, err = tx.ExecContext(ctx, "UPDATE items SET title='Episode',sort_title='episode',year=0,match_key=? WHERE id=?", "unmatch:"+key, key); err != nil {
				return err
			}
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE items SET title=?,sort_title=?,year=? WHERE id=?", title, SortTitle(title), year, id); err != nil {
		return err
	}
	for _, q := range []string{
		"DELETE FROM orderings WHERE series_id=?",
		"DELETE FROM season_art WHERE series_id=?",
		"DELETE FROM series_specials WHERE series_id=?",
		"DELETE FROM episode_specials WHERE episode_id IN (SELECT id FROM items WHERE parent_id=?) OR before_episode_id IN (SELECT id FROM items WHERE parent_id=?)",
	} {
		args := []any{id}
		if strings.Contains(q, "OR before_episode_id") {
			args = append(args, id)
		}
		if _, err = tx.ExecContext(ctx, q, args...); err != nil {
			return err
		}
	}
	if kind == "series" {
		var orderingID int64
		if err = tx.QueryRowContext(ctx, `INSERT INTO orderings(series_id,source,source_ref,name,description,updated_at) VALUES(?,'file','','File Order','From file names',?) RETURNING id`, id, time.Now().Unix()).Scan(&orderingID); err != nil {
			return err
		}
		keys := map[string]bool{}
		for _, ep := range episodes {
			key := ep.Key
			if key == "" || keys[key] {
				key = "unmatch:" + ep.ID
			}
			keys[key] = true
			if _, err = tx.ExecContext(ctx, "UPDATE items SET title=?,sort_title=?,match_key=? WHERE id=?", ep.Title, SortTitle(ep.Title), key, ep.ID); err != nil {
				return err
			}
			if ep.Placed {
				if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO ordering_entries(ordering_id,episode_id,season_number,episode_number,locked) VALUES(?,?,?,?,0)`, orderingID, ep.ID, ep.Season, ep.Episode); err != nil {
					return err
				}
			}
		}
	}
	// Provider-only cached episodes can disappear only when no saved user reference needs them.
	_, err = tx.ExecContext(ctx, `DELETE FROM items WHERE parent_id=?
 AND NOT EXISTS(SELECT 1 FROM item_files WHERE item_id=items.id)
 AND NOT EXISTS(SELECT 1 FROM user_item_data WHERE item_id=items.id)
 AND NOT EXISTS(SELECT 1 FROM play_history WHERE item_id=items.id)
 AND NOT EXISTS(SELECT 1 FROM playlist_entries WHERE item_id=items.id)
 AND NOT EXISTS(SELECT 1 FROM play_queue_entries WHERE item_id=items.id)
 AND NOT EXISTS(SELECT 1 FROM collection_items WHERE item_id=items.id)`, id)
	if err != nil {
		return err
	}
	return tx.Commit()
}
