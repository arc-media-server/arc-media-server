-- The original language of movies and shows, and a show's status and air dates.
ALTER TABLE items ADD COLUMN original_language TEXT NOT NULL DEFAULT '';
ALTER TABLE items ADD COLUMN show_status TEXT NOT NULL DEFAULT '';
ALTER TABLE items ADD COLUMN last_aired TEXT NOT NULL DEFAULT '';
ALTER TABLE items ADD COLUMN next_aired TEXT NOT NULL DEFAULT '';
-- Fetch movies' and shows' metadata again over the next scans to fill them in.
UPDATE items SET synced_at = 0 WHERE kind IN ('movie', 'series') AND metadata_state = 'matched';
