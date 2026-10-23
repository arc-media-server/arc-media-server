-- Format evidence is separate from playback compatibility and tone-mapping decisions.
ALTER TABLE media_files ADD COLUMN format_probe_version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE media_streams ADD COLUMN channel_layout TEXT NOT NULL DEFAULT '';
ALTER TABLE media_streams ADD COLUMN dolby_vision INTEGER NOT NULL DEFAULT 0;
ALTER TABLE media_streams ADD COLUMN dolby_vision_profile INTEGER NOT NULL DEFAULT 0;
ALTER TABLE media_streams ADD COLUMN hdr10_plus INTEGER NOT NULL DEFAULT 0;
ALTER TABLE media_streams ADD COLUMN hdr_probe_status TEXT NOT NULL DEFAULT '';
CREATE INDEX media_files_format_pending ON media_files(library_id, format_probe_version) WHERE missing_since IS NULL;
