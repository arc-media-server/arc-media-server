-- A rowid is not a source identity: SQLite can reuse it, and restoring an older DB can rewind it.
-- Persist a private per-source revision; old unverified cache layouts remain unserved.
ALTER TABLE media_files ADD COLUMN cache_key TEXT NOT NULL DEFAULT '';
UPDATE media_files SET cache_key=lower(hex(randomblob(16)));
