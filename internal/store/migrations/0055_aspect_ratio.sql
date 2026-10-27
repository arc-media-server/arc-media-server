-- Each video track's display aspect ratio (e.g. 1.78 for 16:9, 2.39 for scope), with
-- anamorphic pixels taken into account. 0 means not read yet: the Media analysis task
-- fills it in for older files (format_probe_version 2).
ALTER TABLE media_streams ADD COLUMN aspect_ratio REAL NOT NULL DEFAULT 0;
