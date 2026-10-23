ALTER TABLE media_files ADD COLUMN local_marker_generation INTEGER NOT NULL DEFAULT 0;
CREATE TABLE background_jobs (
 file_id INTEGER NOT NULL REFERENCES media_files(id) ON DELETE CASCADE,
 kind TEXT NOT NULL, job_key TEXT NOT NULL, revision TEXT NOT NULL,
 state TEXT NOT NULL DEFAULT 'queued', error TEXT NOT NULL DEFAULT '',
 updated_at INTEGER NOT NULL DEFAULT 0,
 PRIMARY KEY(kind,job_key)
);
INSERT OR IGNORE INTO settings(key,value) VALUES ('background_epoch',strftime('%s','now'));
