-- Optional provider-supplied actor appearance counts; 0 means unknown, not zero appearances.
ALTER TABLE item_people ADD COLUMN episode_count INTEGER NOT NULL DEFAULT 0 CHECK (episode_count >= 0);

-- Hints refer to stable Arc identities, never file names or mutable display numbers.
CREATE TABLE episode_specials (
    episode_id TEXT PRIMARY KEY REFERENCES items(id) ON DELETE CASCADE,
    before_episode_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE
);
CREATE INDEX episode_specials_target ON episode_specials(before_episode_id);

-- Independent of the chosen ordering, with the same profile/server-default inheritance.
CREATE TABLE series_specials (
    series_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL DEFAULT '',
    inline INTEGER NOT NULL CHECK (inline IN (0, 1)),
    PRIMARY KEY(series_id, user_id)
);
