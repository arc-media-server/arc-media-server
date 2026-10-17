-- Collection fields the admin edited ("title", "sort_title", "overview", "poster", "backdrop"),
-- which TMDB refreshes leave alone.
ALTER TABLE collections ADD COLUMN locked_fields TEXT NOT NULL DEFAULT '';
