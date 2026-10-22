-- Scans check whether cached episodes are still referenced by an ordering or
-- watch history. Existing composite indexes begin with ordering/user IDs.
CREATE INDEX ordering_entries_episode ON ordering_entries (episode_id);
CREATE INDEX user_item_data_item ON user_item_data (item_id);
