-- Smart playlists, as in Plex: built from a library filter (JSON, like a smart collection's) and
-- kept up to date, instead of a list of entries.
ALTER TABLE playlists ADD COLUMN filter TEXT NOT NULL DEFAULT '';
