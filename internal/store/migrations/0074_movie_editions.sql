-- NULL marks movie rows that predate edition-aware scanning.
ALTER TABLE items ADD COLUMN movie_edition TEXT;
ALTER TABLE items ADD COLUMN movie_family TEXT NOT NULL DEFAULT '';
CREATE INDEX items_movie_family ON items(library_id, movie_family) WHERE kind = 'movie';
