-- A queue belongs to a profile on one authenticated device/session, not to a whole household.
CREATE TABLE play_queues (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 session_hash TEXT NOT NULL REFERENCES sessions(token_hash) ON DELETE CASCADE,
 revision INTEGER NOT NULL DEFAULT 1,
 current_entry_id TEXT NOT NULL DEFAULT '',
 shuffled INTEGER NOT NULL DEFAULT 0,
 expires_at INTEGER NOT NULL,
 UNIQUE(user_id,session_hash)
);
CREATE INDEX play_queues_session ON play_queues(session_hash);
CREATE INDEX play_queues_expiry ON play_queues(expires_at);
CREATE TABLE play_queue_entries (
 id TEXT PRIMARY KEY,
 queue_id TEXT NOT NULL REFERENCES play_queues(id) ON DELETE CASCADE,
 item_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
 position INTEGER NOT NULL,
 original_position INTEGER NOT NULL
);
CREATE INDEX play_queue_entries_queue ON play_queue_entries(queue_id,position);
CREATE INDEX play_queue_entries_item ON play_queue_entries(item_id);
