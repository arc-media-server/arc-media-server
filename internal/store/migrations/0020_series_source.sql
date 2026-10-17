-- A show can override its library's episode numbering ("tmdb_aired", "tvdb_aired", "tvdb_dvd",
-- "tvdb_absolute"); '' follows the library.
ALTER TABLE items ADD COLUMN source_ordering TEXT NOT NULL DEFAULT '';
