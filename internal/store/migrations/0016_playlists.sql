-- Each profile's own playlists of movies and episodes, played in order.
CREATE TABLE playlists (
    id         TEXT    PRIMARY KEY,
    user_id    TEXT    NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title      TEXT    NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE INDEX playlists_user ON playlists (user_id);

-- A title can be in a playlist more than once, so entries have their own ids.
CREATE TABLE playlist_entries (
    id          INTEGER PRIMARY KEY,
    playlist_id TEXT    NOT NULL REFERENCES playlists (id) ON DELETE CASCADE,
    item_id     TEXT    NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    position    INTEGER NOT NULL
);
CREATE INDEX playlist_entries_playlist ON playlist_entries (playlist_id, position);
