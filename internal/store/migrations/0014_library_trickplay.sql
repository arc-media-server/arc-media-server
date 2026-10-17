-- Whether a library gets thumbnails for scrubbing along the player's timeline. Off by default, as
-- making them takes time and disk space.
ALTER TABLE libraries ADD COLUMN trickplay INTEGER NOT NULL DEFAULT 0;
