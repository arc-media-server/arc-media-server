-- Other videos libraries (home videos). The original table only allows kind 'movies' or 'shows', so
-- they're stored as 'movies' with this flag and read back as kind 'videos'. Their videos use
-- metadata_state 'locked': never looked up online.
ALTER TABLE libraries ADD COLUMN videos INTEGER NOT NULL DEFAULT 0;
