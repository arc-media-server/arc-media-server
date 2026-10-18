-- Friends: people outside the household, with their own name and password, who sign in from
-- anywhere and never appear on the household's profile picker. They join with an invite link.
ALTER TABLE users ADD COLUMN is_friend INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN invite_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN invite_expires INTEGER NOT NULL DEFAULT 0;

-- A friend's sessions are started with their own password: via 'friend'. SQLite can't change a
-- CHECK, so the table is rebuilt (keeping everyone signed in).
CREATE TABLE sessions_new (
    token_hash   TEXT    PRIMARY KEY,
    user_id      TEXT    REFERENCES users (id) ON DELETE CASCADE,
    -- password: signed in with the admin password. local: started without one on the home
    -- network, so it only works there and only while that's allowed. friend: a friend's own password.
    via          TEXT    NOT NULL CHECK (via IN ('password', 'local', 'friend')),
    user_agent   TEXT    NOT NULL DEFAULT '',
    created_at   INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL
);
INSERT INTO sessions_new SELECT token_hash, user_id, via, user_agent, created_at, last_seen_at FROM sessions;
DROP TABLE sessions;
ALTER TABLE sessions_new RENAME TO sessions;
CREATE INDEX sessions_user ON sessions (user_id);
