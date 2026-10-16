-- Arc Media Server initial schema.
--
-- Design rules:
--   * Files on disk are never modified. media_files is a read-only mirror of what the scanner saw.
--   * Items (movies, series, episodes) are logical identities. Watch state and metadata attach to
--     items, never to file paths or season/episode numbers.
--   * Seasons are not items. A season only exists inside an ordering, so the same episode can be
--     S01E05 in TVDB aired order and S02E01 in a TMDB episode group without moving anything.

CREATE TABLE libraries (
    id              INTEGER PRIMARY KEY,
    name            TEXT    NOT NULL UNIQUE,
    kind            TEXT    NOT NULL CHECK (kind IN ('movies', 'shows')),
    -- How season/episode numbers in filenames should be interpreted (shows only).
    source_ordering TEXT    NOT NULL DEFAULT 'tvdb_aired',
    created_at      INTEGER NOT NULL
);

CREATE TABLE library_paths (
    library_id INTEGER NOT NULL REFERENCES libraries (id) ON DELETE CASCADE,
    path       TEXT    NOT NULL,
    PRIMARY KEY (library_id, path)
);

CREATE TABLE items (
    id             TEXT    PRIMARY KEY, -- 32 hex chars, maps cleanly onto Jellyfin GUIDs later
    library_id     INTEGER NOT NULL REFERENCES libraries (id) ON DELETE CASCADE,
    kind           TEXT    NOT NULL CHECK (kind IN ('movie', 'series', 'episode')),
    parent_id      TEXT    REFERENCES items (id) ON DELETE CASCADE,
    -- Stable key the scanner uses to find an existing item, e.g. "title:the matrix:1999" or "tmdb:603".
    match_key      TEXT    NOT NULL,
    title          TEXT    NOT NULL,
    sort_title     TEXT    NOT NULL,
    original_title TEXT    NOT NULL DEFAULT '',
    year           INTEGER NOT NULL DEFAULT 0,
    overview       TEXT    NOT NULL DEFAULT '',
    runtime_min    INTEGER NOT NULL DEFAULT 0,
    release_date   TEXT    NOT NULL DEFAULT '',
    tmdb_id        INTEGER NOT NULL DEFAULT 0,
    tvdb_id        INTEGER NOT NULL DEFAULT 0,
    imdb_id        TEXT    NOT NULL DEFAULT '',
    poster_path    TEXT    NOT NULL DEFAULT '',
    backdrop_path  TEXT    NOT NULL DEFAULT '',
    metadata_state TEXT    NOT NULL DEFAULT 'pending'
        CHECK (metadata_state IN ('pending', 'matched', 'unmatched', 'locked')),
    added_at       INTEGER NOT NULL,
    updated_at     INTEGER NOT NULL,
    UNIQUE (library_id, match_key)
);
CREATE INDEX items_library_kind ON items (library_id, kind, sort_title);
CREATE INDEX items_parent ON items (parent_id);

CREATE TABLE media_files (
    id            INTEGER PRIMARY KEY,
    library_id    INTEGER NOT NULL REFERENCES libraries (id) ON DELETE CASCADE,
    path          TEXT    NOT NULL UNIQUE,
    size          INTEGER NOT NULL,
    mtime         INTEGER NOT NULL,
    container     TEXT    NOT NULL DEFAULT '',
    duration_ms   INTEGER NOT NULL DEFAULT 0,
    bitrate       INTEGER NOT NULL DEFAULT 0,
    probed_at     INTEGER,
    probe_error   TEXT    NOT NULL DEFAULT '',
    last_seen_at  INTEGER NOT NULL,
    -- Set when a scan no longer finds the file. Items are kept so watch history survives
    -- a drive being offline or a file being replaced.
    missing_since INTEGER
);
CREATE INDEX media_files_library ON media_files (library_id, last_seen_at);

CREATE TABLE media_streams (
    file_id    INTEGER NOT NULL REFERENCES media_files (id) ON DELETE CASCADE,
    idx        INTEGER NOT NULL,
    kind       TEXT    NOT NULL, -- video, audio, subtitle, attachment, data
    codec      TEXT    NOT NULL DEFAULT '',
    profile    TEXT    NOT NULL DEFAULT '',
    language   TEXT    NOT NULL DEFAULT '',
    title      TEXT    NOT NULL DEFAULT '',
    width      INTEGER NOT NULL DEFAULT 0,
    height     INTEGER NOT NULL DEFAULT 0,
    channels   INTEGER NOT NULL DEFAULT 0,
    is_default INTEGER NOT NULL DEFAULT 0,
    is_forced  INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (file_id, idx)
);

-- Many-to-many: one file can hold several episodes (S01E01-E02.mkv) and one item can span
-- several files (cd1/cd2).
CREATE TABLE item_files (
    item_id TEXT    NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    file_id INTEGER NOT NULL REFERENCES media_files (id) ON DELETE CASCADE,
    part    INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (item_id, file_id)
);
CREATE INDEX item_files_file ON item_files (file_id);

-- An ordering is one way of arranging a series' episodes into seasons.
CREATE TABLE orderings (
    id         INTEGER PRIMARY KEY,
    series_id  TEXT    NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    -- tvdb_aired, tvdb_dvd, tvdb_absolute, tmdb_aired, tmdb_group, custom
    source     TEXT    NOT NULL,
    -- Provider-specific reference, e.g. the TMDB episode group id. Empty for built-in orders.
    source_ref TEXT    NOT NULL DEFAULT '',
    name       TEXT    NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE (series_id, source, source_ref)
);

CREATE TABLE ordering_seasons (
    ordering_id   INTEGER NOT NULL REFERENCES orderings (id) ON DELETE CASCADE,
    season_number INTEGER NOT NULL,
    name          TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (ordering_id, season_number)
);

CREATE TABLE ordering_entries (
    ordering_id    INTEGER NOT NULL REFERENCES orderings (id) ON DELETE CASCADE,
    episode_id     TEXT    NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    season_number  INTEGER NOT NULL,
    episode_number INTEGER NOT NULL,
    -- Manual overrides survive metadata refreshes.
    locked         INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (ordering_id, episode_id, season_number, episode_number)
);
CREATE INDEX ordering_entries_position ON ordering_entries (ordering_id, season_number, episode_number);

CREATE TABLE users (
    id            TEXT    PRIMARY KEY,
    name          TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    password_hash TEXT    NOT NULL DEFAULT '',
    is_admin      INTEGER NOT NULL DEFAULT 0,
    created_at    INTEGER NOT NULL
);

-- Which ordering a series is displayed in. user_id '' is the server-wide default;
-- a row with a real user id overrides it for that user.
CREATE TABLE series_display (
    series_id   TEXT    NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    user_id     TEXT    NOT NULL DEFAULT '',
    ordering_id INTEGER NOT NULL REFERENCES orderings (id) ON DELETE CASCADE,
    PRIMARY KEY (series_id, user_id)
);

CREATE TABLE user_item_data (
    user_id        TEXT    NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    item_id        TEXT    NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    position_ms    INTEGER NOT NULL DEFAULT 0,
    played         INTEGER NOT NULL DEFAULT 0,
    play_count     INTEGER NOT NULL DEFAULT 0,
    last_played_at INTEGER,
    PRIMARY KEY (user_id, item_id)
);
