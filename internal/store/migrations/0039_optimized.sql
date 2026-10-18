-- Optimized versions: easy-to-play copies (H.264/AAC MP4, at most 1080p or 720p) made in Arc's data
-- folder, never beside the media. status: queued, converting, done or failed.
CREATE TABLE optimized_versions (
    file_id    INTEGER NOT NULL REFERENCES media_files (id) ON DELETE CASCADE,
    quality    INTEGER NOT NULL,
    path       TEXT    NOT NULL DEFAULT '',
    size       INTEGER NOT NULL DEFAULT 0,
    status     TEXT    NOT NULL,
    error      TEXT    NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    PRIMARY KEY (file_id, quality)
);
