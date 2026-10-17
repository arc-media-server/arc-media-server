-- Each profile's own preferences, such as its home page layout and spoiler protection, as JSON.
ALTER TABLE users ADD COLUMN prefs TEXT NOT NULL DEFAULT '{}';
