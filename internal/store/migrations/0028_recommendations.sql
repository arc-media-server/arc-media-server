-- TMDB's recommendations for a movie or show (by TMDB id, best first), for "More like this".
CREATE TABLE item_recommendations (
    item_id TEXT    NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    rank    INTEGER NOT NULL,
    tmdb_id INTEGER NOT NULL,
    PRIMARY KEY (item_id, rank)
);
-- Fetch movies' and shows' metadata again over the next scans so they get recommendations.
UPDATE items SET synced_at = 0 WHERE kind IN ('movie', 'series') AND metadata_state = 'matched';
