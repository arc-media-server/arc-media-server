-- Season posters: from TMDB, or a seasonNN-poster.jpg in the show's folder. Season numbers are
-- the provider's aired seasons.
CREATE TABLE season_art (
    series_id     TEXT    NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    season_number INTEGER NOT NULL,
    poster_path   TEXT    NOT NULL DEFAULT '',
    local_poster  TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (series_id, season_number)
);
-- Fetch shows' metadata again over the next scans so they get their season posters.
UPDATE items SET synced_at = 0 WHERE kind = 'series' AND metadata_state = 'matched';
