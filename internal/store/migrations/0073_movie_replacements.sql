-- Existing files have no recorded discovery time. Leave it unknown so a migration
-- cannot mistake an existing alternate version for a new file.
ALTER TABLE media_files ADD COLUMN first_seen_at INTEGER;
ALTER TABLE items ADD COLUMN movie_replaced_at INTEGER;
ALTER TABLE items ADD COLUMN movie_replacement_sequence INTEGER;
