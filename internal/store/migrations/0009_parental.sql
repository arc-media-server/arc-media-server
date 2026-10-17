-- Parental controls.

-- The minimum age an item's age rating means (from content_rating), -1 when unrated or unknown.
-- Kept in step by the store whenever content_rating is written.
ALTER TABLE items ADD COLUMN min_age INTEGER NOT NULL DEFAULT -1;

-- The oldest age rating a profile may watch; 0 means no limit. Unrated titles are hidden from
-- profiles with a limit.
ALTER TABLE users ADD COLUMN max_age INTEGER NOT NULL DEFAULT 0;
