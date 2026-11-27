package store

import (
	"context"
	"strings"
)

func movieEditionNameKey(edition string) string {
	name := strings.ToLower(edition)
	if name == "theatrical" {
		return "theatrical cut"
	}
	return name
}

func MovieEditionKey(family, edition string) string {
	if edition == "" {
		return family
	}
	return "edition:" + family + "\x1f" + movieEditionNameKey(edition)
}

// ReconcileMovieEditions splits pre-existing movie entries on the next scan.
// The original entry keeps its ID and all history. An unlabelled copy takes
// precedence. Otherwise the first primary file recorded retains the history.
func (s *Store) ReconcileMovieEditions(ctx context.Context, libraryID int64) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id, match_key FROM items
		WHERE library_id = ? AND kind = 'movie' AND movie_edition IS NULL ORDER BY id`, libraryID)
	if err != nil {
		return err
	}
	type legacyMovie struct{ id, family string }
	var pending []legacyMovie
	for rows.Next() {
		var movie legacyMovie
		if err := rows.Scan(&movie.id, &movie.family); err != nil {
			rows.Close()
			return err
		}
		pending = append(pending, movie)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, movie := range pending {
		if err := s.reconcileMovieEditions(ctx, movie.id, movie.family); err != nil {
			return err
		}
	}
	return nil
}

type editionFile struct {
	id       int64
	part     int
	path     string
	override string
}

type movieEdition struct {
	name  string
	files []editionFile
}

func (s *Store) reconcileMovieEditions(ctx context.Context, id, family string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT mf.id, f.part, mf.path, COALESCE(mf.item_override, '')
		FROM item_files f JOIN media_files mf ON mf.id=f.file_id WHERE f.item_id=?
		ORDER BY CASE WHEN f.part <= 1 THEN 0 ELSE 1 END, mf.id`, id)
	if err != nil {
		return err
	}
	var groups []movieEdition
	indexes := map[string]int{}
	manual := strings.HasPrefix(family, "split:")
	for rows.Next() {
		var file editionFile
		if err := rows.Scan(&file.id, &file.part, &file.path, &file.override); err != nil {
			rows.Close()
			return err
		}
		manual = manual || file.override != ""
		name := EditionOf(file.path)
		key := movieEditionNameKey(name)
		index, ok := indexes[key]
		if !ok {
			index = len(groups)
			indexes[key] = index
			groups = append(groups, movieEdition{name: name})
		}
		groups[index].files = append(groups[index].files, file)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}

	// Leave ambiguous multipart groups together. A cut without a primary file
	// cannot be placed safely using its filename alone.
	if len(groups) > 1 {
		for _, group := range groups {
			hasPrimary := false
			for _, file := range group.files {
				hasPrimary = hasPrimary || file.part <= 1
			}
			manual = manual || !hasPrimary
		}
	}
	// Preserve existing overrides and pin all files in these groups so discovery
	// does not move an unoverridden part later in the same scan.
	if manual || len(groups) == 0 {
		edition := ""
		if len(groups) == 1 {
			edition = groups[0].name
		}
		if _, err := tx.ExecContext(ctx, `UPDATE items SET movie_edition=? WHERE id=? AND movie_edition IS NULL`, edition, id); err != nil {
			return err
		}
		if manual {
			if _, err := tx.ExecContext(ctx, `UPDATE media_files SET item_override=?
				WHERE item_override='' AND id IN (SELECT file_id FROM item_files WHERE item_id=?)`, id, id); err != nil {
				return err
			}
		}
		return tx.Commit()
	}
	keep := 0
	if standard, exists := indexes[""]; exists {
		keep = standard
	}
	retained := groups[keep]
	result, err := tx.ExecContext(ctx, `UPDATE items SET match_key=?, movie_edition=?, movie_family=?
		WHERE id=? AND movie_edition IS NULL`, MovieEditionKey(family, retained.name), retained.name, family, id)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil || n == 0 {
		return err
	}
	if retained.name != "" {
		// Plain aliases must not route a future unlabelled file to a named cut.
		if _, err := tx.ExecContext(ctx, `UPDATE item_aliases SET match_key='edition:' || match_key || ? WHERE item_id=?`,
			"\x1f"+movieEditionNameKey(retained.name), id); err != nil {
			return err
		}
	}

	for i, group := range groups {
		if i == keep {
			continue
		}
		newID := NewID()
		if _, err := tx.ExecContext(ctx, `INSERT INTO items
			(id, library_id, kind, match_key, title, sort_title, original_title, year, overview, runtime_min,
			 release_date, tmdb_id, tvdb_id, imdb_id, poster_path, backdrop_path, metadata_state, added_at, updated_at,
			 locked_fields, tagline, content_rating, studio, rating, local_poster, local_backdrop, logo_path, local_logo,
			 original_language, imdb_rating, imdb_votes, rt_critics, metacritic, local_theme, match_suppressed, synced_at,
			 movie_edition, movie_family)
			SELECT ?, library_id, kind, ?, title, sort_title, original_title, year, overview, runtime_min,
			 release_date, tmdb_id, tvdb_id, imdb_id, poster_path, backdrop_path, metadata_state, added_at, updated_at,
			 locked_fields, tagline, content_rating, studio, rating, local_poster, local_backdrop, logo_path, local_logo,
			 original_language, imdb_rating, imdb_votes, rt_critics, metacritic, local_theme, match_suppressed, synced_at,
			 ?, ? FROM items WHERE id=?`, newID, MovieEditionKey(family, group.name), group.name, family, id); err != nil {
			return err
		}
		// Preserve access labels and shared title metadata. Profile activity belongs only to the original.
		for _, query := range []string{
			`INSERT INTO item_labels(item_id,label) SELECT ?,label FROM item_labels WHERE item_id=?`,
			`INSERT INTO item_genres(item_id,name) SELECT ?,name FROM item_genres WHERE item_id=?`,
			`INSERT INTO item_people(item_id,person_id,role,character,ord,episode_count) SELECT ?,person_id,role,character,ord,episode_count FROM item_people WHERE item_id=?`,
			`INSERT INTO item_alt_titles(item_id,title) SELECT ?,title FROM item_alt_titles WHERE item_id=?`,
			`INSERT INTO item_keywords(item_id,keyword) SELECT ?,keyword FROM item_keywords WHERE item_id=?`,
			`INSERT INTO collection_items(item_id,collection_id,position) SELECT ?,collection_id,position FROM collection_items WHERE item_id=?`,
		} {
			if _, err := tx.ExecContext(ctx, query, newID, id); err != nil {
				return err
			}
		}
		if err := copyArrival(ctx, tx, newID, id); err != nil {
			return err
		}
		for _, file := range group.files {
			if _, err := tx.ExecContext(ctx, `UPDATE item_files SET item_id=? WHERE item_id=? AND file_id=?`, newID, id, file.id); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// OtherMovieEditions returns the other playable cuts of this movie in its library.
func (s *Store) OtherMovieEditions(ctx context.Context, movie Item) ([]Item, error) {
	return s.queryItems(ctx, `SELECT `+prefixCols("i.", itemCols)+` FROM items i
		WHERE i.library_id=? AND i.kind='movie' AND i.id<>?
		AND LOWER(COALESCE(i.movie_edition,''))<>LOWER(?)
		AND ((?<>'' AND i.movie_family=?) OR (?<>0 AND i.tmdb_id=?) OR (?<>'' AND i.imdb_id=?))
		AND EXISTS (SELECT 1 FROM item_files f JOIN media_files m ON m.id=f.file_id
		            WHERE f.item_id=i.id AND f.part<=1 AND m.missing_since IS NULL)
		ORDER BY COALESCE(i.movie_edition,''), i.id`,
		movie.LibraryID, movie.ID, movie.Edition, movie.MovieFamily, movie.MovieFamily,
		movie.TMDBID, movie.TMDBID, movie.IMDBID, movie.IMDBID)
}

func (s *Store) UpdateMovieEdition(ctx context.Context, id, key, family, edition string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE items SET match_key=?, movie_family=?, movie_edition=? WHERE id=?`, key, family, edition, id)
	return err
}
