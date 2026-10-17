package store

import (
	"context"
	"time"
)

// FileProblem is a file AMS couldn't read, recognise or find.
type FileProblem struct {
	FileID int64  `json:"file_id"`
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Error  string `json:"error,omitempty"`
	// When the file was last seen, for missing files (unix seconds).
	MissingSince int64 `json:"missing_since,omitempty"`
	// The item the file belongs to, if any.
	ItemID    string `json:"item_id,omitempty"`
	ItemTitle string `json:"item_title,omitempty"`
	ItemKind  string `json:"item_kind,omitempty"`
	// For episodes: their series.
	SeriesID string `json:"series_id,omitempty"`
}

// fileProblems lists a library's files matching cond (a condition on media_files m, with its
// own parameters), with the first item each belongs to.
func (s *Store) fileProblems(ctx context.Context, libraryID int64, cond string, args ...any) ([]FileProblem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.path, m.size, m.probe_error, COALESCE(m.missing_since, 0),
			COALESCE(i.id, ''), COALESCE(i.title, ''), COALESCE(i.kind, ''), COALESCE(i.parent_id, '')
		FROM media_files m
		LEFT JOIN items i ON i.id = (SELECT f.item_id FROM item_files f WHERE f.file_id = m.id LIMIT 1)
		WHERE m.library_id = ? AND `+cond+`
		ORDER BY m.path`, append([]any{libraryID}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FileProblem{}
	for rows.Next() {
		var p FileProblem
		if err := rows.Scan(&p.FileID, &p.Path, &p.Size, &p.Error, &p.MissingSince,
			&p.ItemID, &p.ItemTitle, &p.ItemKind, &p.SeriesID); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// UnreadableFiles are present files ffprobe couldn't read.
func (s *Store) UnreadableFiles(ctx context.Context, libraryID int64) ([]FileProblem, error) {
	return s.fileProblems(ctx, libraryID, `m.missing_since IS NULL AND m.probe_error <> ''`)
}

// MissingFiles are files a scan no longer finds. Their items are kept (and hidden while none of
// their files are present) so history survives a drive being offline.
func (s *Store) MissingFiles(ctx context.Context, libraryID int64) ([]FileProblem, error) {
	return s.fileProblems(ctx, libraryID, `m.missing_since IS NOT NULL`)
}

// UnrecognisedFiles are present video files that don't belong to any item, e.g. a file in a TV
// library whose name doesn't look like an episode.
func (s *Store) UnrecognisedFiles(ctx context.Context, libraryID int64) ([]FileProblem, error) {
	return s.fileProblems(ctx, libraryID,
		`m.missing_since IS NULL AND NOT EXISTS (SELECT 1 FROM item_files f WHERE f.file_id = m.id)
			AND NOT EXISTS (SELECT 1 FROM extras x WHERE x.file_id = m.id)`)
}

// goneItemsSQL selects the movies, episodes and series in library ?1 that have files, none of
// them present any more.
const goneItemsSQL = `
	SELECT i.id FROM items i
	WHERE i.library_id = ?1 AND i.kind IN ('movie', 'episode')
		AND EXISTS (SELECT 1 FROM item_files f WHERE f.item_id = i.id)
		AND NOT EXISTS (SELECT 1 FROM item_files f JOIN media_files m ON m.id = f.file_id
			WHERE f.item_id = i.id AND m.missing_since IS NULL)
	UNION
	SELECT s.id FROM items s
	WHERE s.library_id = ?1 AND s.kind = 'series'
		AND EXISTS (SELECT 1 FROM items e JOIN item_files f ON f.item_id = e.id WHERE e.parent_id = s.id)
		AND NOT EXISTS (SELECT 1 FROM items e JOIN item_files f ON f.item_id = e.id
			JOIN media_files m ON m.id = f.file_id WHERE e.parent_id = s.id AND m.missing_since IS NULL)`

// EmptyTrash forgets everything in a library whose files are gone: the missing files, and the
// movies, episodes and shows left with no file at all (with their watch history). Titles with at
// least one file still present stay. It returns how many titles were removed.
func (s *Store) EmptyTrash(ctx context.Context, libraryID int64) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `DELETE FROM items WHERE id IN (`+goneItemsSQL+`)`, libraryID)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	if _, err := tx.ExecContext(ctx, `DELETE FROM media_files WHERE library_id = ? AND missing_since IS NOT NULL`, libraryID); err != nil {
		return 0, err
	}
	return n, tx.Commit()
}

// RemoveGoneItem forgets one movie, episode or show whose files are all gone. It returns false
// (and removes nothing) if any of its files is still present.
func (s *Store) RemoveGoneItem(ctx context.Context, it *Item) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var gone int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM (`+goneItemsSQL+`) WHERE id = ?2`, it.LibraryID, it.ID).Scan(&gone); err != nil {
		return false, err
	}
	if gone == 0 {
		return false, nil
	}
	// Its missing files go too (files shared with other titles, like double episodes, are kept).
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM media_files WHERE missing_since IS NOT NULL AND id IN (
			SELECT f.file_id FROM item_files f JOIN items i ON i.id = f.item_id
			WHERE i.id = ?1 OR i.parent_id = ?1)
		AND NOT EXISTS (SELECT 1 FROM item_files f JOIN items i ON i.id = f.item_id
			WHERE f.file_id = media_files.id AND i.id <> ?1 AND COALESCE(i.parent_id, '') <> ?1)`, it.ID); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM items WHERE id = ?`, it.ID); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

// Duplicate is something the library has more than once.
type Duplicate struct {
	// copies: one movie or episode with several files; same_title: several movies or shows
	// identified as the same title.
	Kind  string `json:"kind"`
	Items []Item `json:"items"`
	// For copies: the files.
	Files []FileProblem `json:"files,omitempty"`
}

// Duplicates finds movies and episodes with several present copies, and movies or shows that
// were identified as the same title (e.g. two folders of the same show).
func (s *Store) Duplicates(ctx context.Context, libraryID int64) ([]Duplicate, error) {
	out := []Duplicate{}

	// Several files for the same item and part.
	rows, err := s.db.QueryContext(ctx, `
		SELECT f.item_id FROM item_files f
		JOIN media_files m ON m.id = f.file_id JOIN items i ON i.id = f.item_id
		WHERE i.library_id = ? AND m.missing_since IS NULL
		GROUP BY f.item_id, f.part HAVING COUNT(*) > 1
		ORDER BY i.sort_title`, libraryID)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		it, err := s.GetItem(ctx, id)
		if err != nil || it == nil {
			return nil, err
		}
		files, err := s.fileProblems(ctx, libraryID,
			`m.missing_since IS NULL AND m.id IN (SELECT file_id FROM item_files WHERE item_id = ?)`, id)
		if err != nil {
			return nil, err
		}
		out = append(out, Duplicate{Kind: "copies", Items: []Item{*it}, Files: files})
	}

	// Separate movies or shows identified as the same title.
	for _, q := range []struct{ kind, col string }{{"movie", "tmdb_id"}, {"series", "tvdb_id"}, {"series", "tmdb_id"}} {
		rows, err := s.db.QueryContext(ctx, `SELECT `+itemCols+` FROM items WHERE library_id = ? AND kind = ? AND `+q.col+` IN (
				SELECT `+q.col+` FROM items WHERE library_id = ? AND kind = ? AND `+q.col+` <> 0
				GROUP BY `+q.col+` HAVING COUNT(*) > 1)
			ORDER BY `+q.col+`, added_at`, libraryID, q.kind, libraryID, q.kind)
		if err != nil {
			return nil, err
		}
		var groups []Duplicate
		var last int64 = -1
		for rows.Next() {
			var it Item
			if err := scanItem(rows, &it); err != nil {
				rows.Close()
				return nil, err
			}
			key := it.TMDBID
			if q.col == "tvdb_id" {
				key = it.TVDBID
			}
			if key != last {
				groups = append(groups, Duplicate{Kind: "same_title"})
				last = key
			}
			groups[len(groups)-1].Items = append(groups[len(groups)-1].Items, it)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		for _, g := range groups {
			// A pair of shows sharing both ids is found twice; report it once.
			key := ""
			for _, it := range g.Items {
				key += it.ID + ","
			}
			if !seen[key] {
				seen[key] = true
				out = append(out, g)
			}
		}
	}
	return out, nil
}

// ProblemCounts counts, per library, the things that need the user: unidentified items,
// unreadable files and unrecognised files. (Missing files, duplicates and missing episodes are
// often expected, so they aren't counted.)
func (s *Store) ProblemCounts(ctx context.Context) (map[int64]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT library_id, COUNT(*) FROM items
		WHERE metadata_state = 'unmatched' AND (kind <> 'episode' OR parent_id IN
			(SELECT id FROM items WHERE kind = 'series' AND metadata_state <> 'unmatched'))
		GROUP BY library_id
		UNION ALL
		SELECT library_id, COUNT(*) FROM media_files m
		WHERE m.missing_since IS NULL AND (m.probe_error <> ''
			OR NOT EXISTS (SELECT 1 FROM item_files f WHERE f.file_id = m.id)
			AND NOT EXISTS (SELECT 1 FROM extras x WHERE x.file_id = m.id))
		GROUP BY library_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]int{}
	for rows.Next() {
		var lib int64
		var n int
		if err := rows.Scan(&lib, &n); err != nil {
			return nil, err
		}
		out[lib] += n
	}
	return out, rows.Err()
}

// MissingEpisode is an aired episode the library doesn't have.
type MissingEpisode struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Season  int    `json:"season_number"`
	Episode int    `json:"episode_number"`
	AirDate string `json:"air_date"`
}

// SeriesGaps is a series with aired episodes missing from the library, or episode files its
// display ordering doesn't place.
type SeriesGaps struct {
	Series   Item             `json:"series"`
	Ordering string           `json:"ordering"`
	Missing  []MissingEpisode `json:"missing"`
	// How many episodes of the series are in the library.
	Have int `json:"have"`
	// Episodes in the library that the display ordering doesn't include (shown apart on the
	// series page, so they don't disappear).
	Unplaced int `json:"unplaced"`
}

// MissingEpisodes lists, for each identified series with at least one episode file, the episodes
// in its display ordering that have aired but have no file (specials left out), and how many
// episode files that ordering doesn't place.
func (s *Store) MissingEpisodes(ctx context.Context, lib *Library) ([]SeriesGaps, error) {
	series, err := s.LibraryItemsByKind(ctx, lib.ID, "series")
	if err != nil {
		return nil, err
	}
	today := time.Now().Format("2006-01-02")
	out := []SeriesGaps{}
	for _, se := range series {
		if se.MetadataState == "unmatched" {
			continue
		}
		available, err := s.AvailableChildren(ctx, se.ID)
		if err != nil {
			return nil, err
		}
		if len(available) == 0 {
			continue // nothing of it is on disk (e.g. its drive is offline)
		}
		oid, err := s.DisplayOrdering(ctx, se.ID, "", lib.SourceOrdering)
		if err != nil || oid == 0 {
			if err != nil {
				return nil, err
			}
			continue
		}
		o, err := s.GetOrdering(ctx, oid)
		if err != nil {
			return nil, err
		}
		rows, err := s.db.QueryContext(ctx, `
			SELECT i.id, i.title, e.season_number, e.episode_number, i.release_date
			FROM ordering_entries e JOIN items i ON i.id = e.episode_id
			WHERE e.ordering_id = ? AND e.season_number > 0 AND i.release_date <> '' AND i.release_date <= ?
			ORDER BY e.season_number, e.episode_number`, oid, today)
		if err != nil {
			return nil, err
		}
		gaps := SeriesGaps{Series: se, Ordering: o.Name, Missing: []MissingEpisode{}, Have: len(available)}
		seen := map[string]bool{}
		for rows.Next() {
			var m MissingEpisode
			if err := rows.Scan(&m.ID, &m.Title, &m.Season, &m.Episode, &m.AirDate); err != nil {
				rows.Close()
				return nil, err
			}
			if !available[m.ID] && !seen[m.ID] {
				gaps.Missing = append(gaps.Missing, m)
			}
			seen[m.ID] = true
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		entries, err := s.OrderingEntries(ctx, oid)
		if err != nil {
			return nil, err
		}
		placed := map[string]bool{}
		for _, e := range entries {
			placed[e.EpisodeID] = true
		}
		for id := range available {
			if !placed[id] {
				gaps.Unplaced++
			}
		}
		if len(gaps.Missing) > 0 || gaps.Unplaced > 0 {
			out = append(out, gaps)
		}
	}
	return out, nil
}
