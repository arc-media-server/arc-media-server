-- The title a file was put under by hand (merging or splitting), so scans keep it there.
ALTER TABLE media_files ADD COLUMN item_override TEXT NOT NULL DEFAULT '';
