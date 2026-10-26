-- Show information follows its library; episode information follows file numbering.
-- Preserve stored metadata, identities, files and history. No automatic refresh is started.
ALTER TABLE libraries ADD COLUMN details_provider TEXT NOT NULL DEFAULT ''
    CHECK (details_provider IN ('', 'tvdb'));
