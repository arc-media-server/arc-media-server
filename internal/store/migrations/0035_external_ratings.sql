-- Other sites' ratings: IMDb's (from its ratings file), and Rotten Tomatoes' critics' score and
-- Metacritic's (from OMDb; NULL when there's none), with when they were last asked for.
ALTER TABLE items ADD COLUMN imdb_rating REAL NOT NULL DEFAULT 0;
ALTER TABLE items ADD COLUMN imdb_votes INTEGER NOT NULL DEFAULT 0;
ALTER TABLE items ADD COLUMN rt_critics INTEGER;
ALTER TABLE items ADD COLUMN metacritic INTEGER;
ALTER TABLE items ADD COLUMN critics_at INTEGER NOT NULL DEFAULT 0;
