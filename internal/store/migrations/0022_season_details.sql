-- Season summaries (from TMDB, or written by hand), and which season fields the user edited, so
-- refreshes keep them ("name", "overview").
ALTER TABLE ordering_seasons ADD COLUMN overview TEXT NOT NULL DEFAULT '';
ALTER TABLE ordering_seasons ADD COLUMN locked_fields TEXT NOT NULL DEFAULT '';
-- Fetch shows' metadata again over the next scans so their seasons get summaries.
UPDATE items SET synced_at = 0 WHERE kind = 'series' AND metadata_state = 'matched';
