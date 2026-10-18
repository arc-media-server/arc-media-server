-- Addresses Arc posts events to (new titles, playback, problems); events is a JSON list, [] for all.
CREATE TABLE webhooks (
    id         TEXT    PRIMARY KEY,
    name       TEXT    NOT NULL,
    url        TEXT    NOT NULL,
    events     TEXT    NOT NULL DEFAULT '[]',
    created_at INTEGER NOT NULL
);
-- When each profile last looked at its New for You list.
ALTER TABLE users ADD COLUMN news_seen_at INTEGER NOT NULL DEFAULT 0;
