-- Skip Intro and Skip Credits per library, as in Plex (each library's "Enable intro detection" and
-- "Enable credits detection"). On for TV; movies have credits only; Other videos libraries neither.
ALTER TABLE libraries ADD COLUMN skip_intros INTEGER NOT NULL DEFAULT 1;
ALTER TABLE libraries ADD COLUMN skip_credits INTEGER NOT NULL DEFAULT 1;
UPDATE libraries SET skip_intros = 0, skip_credits = 0 WHERE videos = 1;
UPDATE libraries SET skip_intros = 0 WHERE kind = 'movies';
