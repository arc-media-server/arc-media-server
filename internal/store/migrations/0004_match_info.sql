-- How an item was last identified: the searches made and what they found (JSON), so AMS can
-- explain a match or a failure to match. Empty until the item has been looked up.
ALTER TABLE items ADD COLUMN match_info TEXT NOT NULL DEFAULT '';
