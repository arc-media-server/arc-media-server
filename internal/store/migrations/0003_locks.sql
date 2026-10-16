-- Metadata fields the user has edited or locked, comma-separated (e.g. "title,overview").
-- Provider refreshes leave locked fields alone.
ALTER TABLE items ADD COLUMN locked_fields TEXT NOT NULL DEFAULT '';
