-- Chapter marks in media files, for the player's timeline and chapter list.
CREATE TABLE media_chapters (
    file_id  INTEGER NOT NULL REFERENCES media_files (id) ON DELETE CASCADE,
    idx      INTEGER NOT NULL,
    start_ms INTEGER NOT NULL,
    end_ms   INTEGER NOT NULL,
    title    TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (file_id, idx)
);
-- Probe everything again on the next scan so chapters are read.
UPDATE media_files SET probed_at = NULL WHERE probe_error = '';
