-- What Arc needs to decide how a file can be played: pixel format and bit depth (10-bit video),
-- colour transfer and primaries (HDR), frame rate and bitrate.
ALTER TABLE media_streams ADD COLUMN pix_fmt TEXT NOT NULL DEFAULT '';
ALTER TABLE media_streams ADD COLUMN bit_depth INTEGER NOT NULL DEFAULT 0;
ALTER TABLE media_streams ADD COLUMN color_transfer TEXT NOT NULL DEFAULT '';
ALTER TABLE media_streams ADD COLUMN color_primaries TEXT NOT NULL DEFAULT '';
ALTER TABLE media_streams ADD COLUMN frame_rate REAL NOT NULL DEFAULT 0;
ALTER TABLE media_streams ADD COLUMN bitrate INTEGER NOT NULL DEFAULT 0;
-- Probe everything again on the next scan so these are filled in.
UPDATE media_files SET probed_at = NULL WHERE probe_error = '';
