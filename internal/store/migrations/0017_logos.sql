-- Clear logos: the title as transparent artwork, shown on movie and series pages. From TMDB, or
-- a logo.png / clearlogo.png beside the media.
ALTER TABLE items ADD COLUMN logo_path TEXT NOT NULL DEFAULT '';
ALTER TABLE items ADD COLUMN local_logo TEXT NOT NULL DEFAULT '';
-- Fetch movies' and shows' metadata again over the next scans so they get their logos.
UPDATE items SET synced_at = 0 WHERE kind IN ('movie', 'series') AND metadata_state = 'matched';
