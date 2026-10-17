-- The libraries a profile may see, as comma-separated ids; '' means all of them.
ALTER TABLE users ADD COLUMN library_ids TEXT NOT NULL DEFAULT '';
