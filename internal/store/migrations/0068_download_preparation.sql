-- 0064-0067 are reserved by the pending #350 review integration (#418).
ALTER TABLE users ADD COLUMN allow_downloads INTEGER NOT NULL DEFAULT 0 CHECK(allow_downloads IN (0,1));
CREATE TABLE downloads (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 session_hash TEXT NOT NULL REFERENCES sessions(token_hash) ON DELETE CASCADE,
 item_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
 file_id INTEGER NOT NULL REFERENCES media_files(id) ON DELETE CASCADE,
 idempotency_key TEXT NOT NULL,
 revision TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('preparing','ready','failed')),
 reason_code TEXT NOT NULL DEFAULT '',
 source_path TEXT NOT NULL,
 source_size INTEGER NOT NULL,
 source_mtime INTEGER NOT NULL,
 source_mtime_ns INTEGER NOT NULL DEFAULT 0,
 sha256 TEXT NOT NULL DEFAULT '',
 created_at INTEGER NOT NULL,
 UNIQUE(user_id,session_hash,idempotency_key)
);
CREATE INDEX downloads_pending ON downloads(state,created_at,id);
CREATE INDEX downloads_session ON downloads(session_hash,user_id);
CREATE INDEX downloads_item ON downloads(item_id);
CREATE INDEX downloads_file ON downloads(file_id);
