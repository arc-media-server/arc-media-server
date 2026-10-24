-- Persist the exact mapped source track; do not guess from current library defaults.
-- -2 is unknown legacy provenance, -1 is an intentionally silent copy.
ALTER TABLE optimized_versions ADD COLUMN audio_stream INTEGER NOT NULL DEFAULT -2;
ALTER TABLE optimized_versions ADD COLUMN audio_language TEXT NOT NULL DEFAULT '';
ALTER TABLE optimized_versions ADD COLUMN audio_title TEXT NOT NULL DEFAULT '';

-- Keep old files removable and permit explicit regeneration; never batch-rebuild on upgrade.
UPDATE optimized_versions SET status='failed',
    error='Optimize again to record this copy''s audio track.' WHERE status='done';
