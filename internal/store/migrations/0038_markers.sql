-- Where a file's intro, recap and credits are, for Skip Intro and Skip Credits. source: manual,
-- chapters, theintrodb, introdb or detected; the user's own are never replaced.
CREATE TABLE media_markers (
    file_id    INTEGER NOT NULL REFERENCES media_files (id) ON DELETE CASCADE,
    kind       TEXT    NOT NULL,
    start_ms   INTEGER NOT NULL,
    end_ms     INTEGER NOT NULL,
    source     TEXT    NOT NULL,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (file_id, kind)
);
-- When each file's markers were last looked for (0: never).
ALTER TABLE media_files ADD COLUMN markers_checked_at INTEGER NOT NULL DEFAULT 0;
