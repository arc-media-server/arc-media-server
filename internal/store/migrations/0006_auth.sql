-- Profiles (users) and sign-in sessions.
--
-- The household has one admin profile, which has the password. Other profiles are like Plex
-- managed users: no password, an optional PIN, and their own watch history.

ALTER TABLE users ADD COLUMN pin_hash TEXT NOT NULL DEFAULT '';
-- Avatar colour, a token name such as "violet".
ALTER TABLE users ADD COLUMN color TEXT NOT NULL DEFAULT '';

CREATE TABLE sessions (
    -- SHA-256 of the token in the cookie, so a leaked database doesn't leak working sessions.
    token_hash   TEXT    PRIMARY KEY,
    -- The profile in use, or NULL until one is picked.
    user_id      TEXT    REFERENCES users (id) ON DELETE CASCADE,
    -- password: signed in with the admin password. local: started without one on the home
    -- network, so it only works there and only while that's allowed.
    via          TEXT    NOT NULL CHECK (via IN ('password', 'local')),
    user_agent   TEXT    NOT NULL DEFAULT '',
    created_at   INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL
);
CREATE INDEX sessions_user ON sessions (user_id);
