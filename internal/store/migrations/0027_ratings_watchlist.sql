-- Each profile's own rating of a title (1-10, shown as half stars out of five; 0 for none), and
-- when it put the title on its watchlist (NULL when it's not on it).
ALTER TABLE user_item_data ADD COLUMN user_rating INTEGER NOT NULL DEFAULT 0;
ALTER TABLE user_item_data ADD COLUMN watchlisted_at INTEGER;
CREATE INDEX user_item_data_watchlist ON user_item_data (user_id, watchlisted_at) WHERE watchlisted_at IS NOT NULL;
