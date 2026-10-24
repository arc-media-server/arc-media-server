-- Bind generated copies to their saved source and give each queued attempt its own identity.
ALTER TABLE optimized_versions ADD COLUMN source_path TEXT NOT NULL DEFAULT '';
ALTER TABLE optimized_versions ADD COLUMN source_size INTEGER NOT NULL DEFAULT 0;
ALTER TABLE optimized_versions ADD COLUMN source_mtime INTEGER NOT NULL DEFAULT 0;
ALTER TABLE optimized_versions ADD COLUMN source_mtime_ns INTEGER NOT NULL DEFAULT 0;
ALTER TABLE optimized_versions ADD COLUMN job_id TEXT NOT NULL DEFAULT '';

-- A legacy completed copy might already predate its source: its provenance cannot be inferred
-- from today's file row. Keep it removable, but require an explicit Optimize to regenerate it.
UPDATE optimized_versions SET
    source_path = COALESCE((SELECT path FROM media_files WHERE id = file_id), ''),
    source_size = COALESCE((SELECT size FROM media_files WHERE id = file_id), 0),
    source_mtime = COALESCE((SELECT mtime FROM media_files WHERE id = file_id), 0),
    job_id = lower(hex(randomblob(16))),
    error = CASE WHEN status = 'done' THEN 'Optimize again to refresh this older copy.' ELSE error END,
    status = CASE WHEN status = 'done' THEN 'failed' WHEN status = 'converting' THEN 'queued' ELSE status END;
