-- The language a library's titles and descriptions are fetched in ("" for TMDB's default, English).
ALTER TABLE libraries ADD COLUMN metadata_language TEXT NOT NULL DEFAULT '';
