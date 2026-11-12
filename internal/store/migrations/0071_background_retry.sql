-- A finished job can ask to run again later (intro detection for a season that still has episodes
-- without an intro): when, and how many times it already has. Both reset when the files change.
ALTER TABLE background_jobs ADD COLUMN retry_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE background_jobs ADD COLUMN retries INTEGER NOT NULL DEFAULT 0;
