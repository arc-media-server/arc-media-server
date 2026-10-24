-- Measured output metadata, never inferred from the source or the quality preset.
-- Existing copies remain valid without a budget; limited playback rejects unknown values.
ALTER TABLE optimized_versions ADD COLUMN duration_ms INTEGER NOT NULL DEFAULT 0;
ALTER TABLE optimized_versions ADD COLUMN bitrate INTEGER NOT NULL DEFAULT 0;
