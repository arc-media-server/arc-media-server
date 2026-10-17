-- Richer metadata: tagline, age rating, studio or network, score, genres, cast and crew.

ALTER TABLE items ADD COLUMN tagline TEXT NOT NULL DEFAULT '';
-- Age rating for the server's region, e.g. "15" or "PG-13".
ALTER TABLE items ADD COLUMN content_rating TEXT NOT NULL DEFAULT '';
-- Studios (movies) or networks (shows), comma-separated.
ALTER TABLE items ADD COLUMN studio TEXT NOT NULL DEFAULT '';
-- Audience score out of 10 (TMDB), 0 when unknown.
ALTER TABLE items ADD COLUMN rating REAL NOT NULL DEFAULT 0;

CREATE TABLE item_genres (
    item_id TEXT NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    name    TEXT NOT NULL,
    PRIMARY KEY (item_id, name)
);
CREATE INDEX item_genres_name ON item_genres (name);

CREATE TABLE people (
    id           INTEGER PRIMARY KEY,
    tmdb_id      INTEGER NOT NULL UNIQUE,
    name         TEXT    NOT NULL,
    profile_path TEXT    NOT NULL DEFAULT ''
);

CREATE TABLE item_people (
    item_id   TEXT    NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    person_id INTEGER NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    -- actor, director, writer or creator
    role      TEXT    NOT NULL,
    character TEXT    NOT NULL DEFAULT '',
    ord       INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (item_id, person_id, role)
);
CREATE INDEX item_people_person ON item_people (person_id);

-- Fetch every series again so it gets the new details (movies are fetched again because their
-- synced_at is still 0).
UPDATE items SET synced_at = 0 WHERE kind = 'series';
