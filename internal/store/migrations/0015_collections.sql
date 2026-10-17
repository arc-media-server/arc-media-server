-- Collections: automatic ones from TMDB ("The Matrix Collection"), manual ones the admin makes,
-- and smart ones defined by a filter.
ALTER TABLE items ADD COLUMN tmdb_collection_id INTEGER NOT NULL DEFAULT 0;

-- The fewest films a TMDB collection needs before it's shown (0 turns automatic collections off).
ALTER TABLE libraries ADD COLUMN collection_min INTEGER NOT NULL DEFAULT 2;

CREATE TABLE collections (
    id            TEXT    PRIMARY KEY,
    library_id    INTEGER NOT NULL REFERENCES libraries (id) ON DELETE CASCADE,
    kind          TEXT    NOT NULL, -- auto, manual, smart
    tmdb_id       INTEGER NOT NULL DEFAULT 0,
    title         TEXT    NOT NULL,
    sort_title    TEXT    NOT NULL,
    overview      TEXT    NOT NULL DEFAULT '',
    poster_path   TEXT    NOT NULL DEFAULT '',
    backdrop_path TEXT    NOT NULL DEFAULT '',
    -- Smart collections: the filter (JSON, like a custom home row's).
    filter        TEXT    NOT NULL DEFAULT '',
    -- An automatic collection the admin removed stays hidden.
    hidden        INTEGER NOT NULL DEFAULT 0,
    created_at    INTEGER NOT NULL
);
CREATE UNIQUE INDEX collections_tmdb ON collections (library_id, tmdb_id) WHERE tmdb_id <> 0;

-- Manual collections' titles, in order.
CREATE TABLE collection_items (
    collection_id TEXT    NOT NULL REFERENCES collections (id) ON DELETE CASCADE,
    item_id       TEXT    NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    position      INTEGER NOT NULL,
    PRIMARY KEY (collection_id, item_id)
);

-- Fetch identified movies again so their TMDB collection is known.
UPDATE items SET synced_at = 0 WHERE kind = 'movie' AND tmdb_id <> 0;
