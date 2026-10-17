-- Your own labels on movies and shows (e.g. "Not for kids"), for filters, smart collections and
-- hiding titles from profiles.
CREATE TABLE item_labels (
    item_id TEXT NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    label   TEXT NOT NULL COLLATE NOCASE,
    PRIMARY KEY (item_id, label)
);
CREATE INDEX item_labels_label ON item_labels (label);
-- Labels whose titles a profile doesn't see (a JSON list).
ALTER TABLE users ADD COLUMN exclude_labels TEXT NOT NULL DEFAULT '[]';
