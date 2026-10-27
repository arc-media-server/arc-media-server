-- Removing a library or emptying trash deletes many items and files at once. Every deleted
-- row is looked up in each table that refers to it, so these columns need an index or each
-- lookup reads the whole table (a large TV library took minutes to remove).
CREATE INDEX IF NOT EXISTS background_jobs_file ON background_jobs (file_id);
CREATE INDEX IF NOT EXISTS collection_items_item ON collection_items (item_id);
CREATE INDEX IF NOT EXISTS playlist_entries_item ON playlist_entries (item_id);
CREATE INDEX IF NOT EXISTS continue_dismissed_item ON continue_dismissed (item_id);
CREATE INDEX IF NOT EXISTS series_display_ordering ON series_display (ordering_id);
