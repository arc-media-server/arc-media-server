-- Artwork found next to the media (poster.jpg, fanart.jpg, "<file>-thumb.jpg"...). Kept apart
-- from the provider's so refreshes don't touch it; shown in preference to the provider's.
ALTER TABLE items ADD COLUMN local_poster TEXT NOT NULL DEFAULT '';
ALTER TABLE items ADD COLUMN local_backdrop TEXT NOT NULL DEFAULT '';
