-- A theme tune found next to a show or movie (theme.mp3 and the like), played quietly on its page.
ALTER TABLE items ADD COLUMN local_theme TEXT NOT NULL DEFAULT '';
