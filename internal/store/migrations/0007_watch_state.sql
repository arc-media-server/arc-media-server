-- Finding what each profile is in the middle of, and watched most recently.
CREATE INDEX user_item_data_recent ON user_item_data (user_id, last_played_at);
