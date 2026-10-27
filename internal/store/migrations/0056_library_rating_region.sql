-- The country whose age ratings a library uses (ISO 3166-1, e.g. "GB"), as Plex's per-library
-- Country. Existing libraries keep the server-wide region they used until now.
ALTER TABLE libraries ADD COLUMN rating_region TEXT NOT NULL DEFAULT '';
UPDATE libraries SET rating_region = COALESCE((SELECT value FROM settings WHERE key = 'region' AND value <> ''), 'US');
