-- Other names a movie or show is known by (e.g. "Sen to Chihiro"), found by search.
CREATE TABLE item_alt_titles (
    item_id TEXT NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    title   TEXT NOT NULL,
    PRIMARY KEY (item_id, title)
);
-- TMDB keywords (e.g. "christmas"), for filters, smart collections and seasonal rows.
CREATE TABLE item_keywords (
    item_id TEXT NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    keyword TEXT NOT NULL,
    PRIMARY KEY (item_id, keyword)
);
CREATE INDEX item_keywords_keyword ON item_keywords (keyword);
-- Fetch movies' and shows' metadata again over the next scans to fill them in.
UPDATE items SET synced_at = 0 WHERE kind IN ('movie', 'series') AND metadata_state = 'matched';
