-- Every viewing of a movie or episode, for watch history and stats. A viewing is recorded once
-- it has played for a minute. What was watched is copied in (title, show, library), so history
-- and stats survive the item being removed from the library.
CREATE TABLE play_history (
    id           INTEGER PRIMARY KEY,
    user_id      TEXT    NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    item_id      TEXT    NOT NULL,
    kind         TEXT    NOT NULL,             -- movie or episode
    library_id   INTEGER NOT NULL,
    series_id    TEXT    NOT NULL DEFAULT '',  -- an episode's show
    title        TEXT    NOT NULL,
    series_title TEXT    NOT NULL DEFAULT '',
    started_at   INTEGER NOT NULL,
    ended_at     INTEGER NOT NULL,
    -- Time actually spent playing (not paused), and how far through it got.
    watched_ms   INTEGER NOT NULL DEFAULT 0,
    position_ms  INTEGER NOT NULL DEFAULT 0,
    duration_ms  INTEGER NOT NULL DEFAULT 0,
    completed    INTEGER NOT NULL DEFAULT 0,
    device       TEXT    NOT NULL DEFAULT '',
    method       TEXT    NOT NULL DEFAULT ''   -- direct, remux or transcode
);
CREATE INDEX play_history_started ON play_history (started_at);
CREATE INDEX play_history_user ON play_history (user_id, started_at);
CREATE INDEX play_history_item ON play_history (item_id);
