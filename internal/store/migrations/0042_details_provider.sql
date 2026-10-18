-- Where a show's details and artwork come from: '' for TMDB (the default), or 'tvdb'. One provider
-- only; gaps aren't filled from the other. Separate from how its files are numbered.
ALTER TABLE items ADD COLUMN details_provider TEXT NOT NULL DEFAULT '';
