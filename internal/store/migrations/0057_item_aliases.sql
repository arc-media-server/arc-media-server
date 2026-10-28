-- Another folder name's key that now belongs to a title, after shows were merged: the scanner
-- files that folder's episodes under the kept show instead of making the merged one again.
CREATE TABLE item_aliases (
    library_id INTEGER NOT NULL REFERENCES libraries (id) ON DELETE CASCADE,
    match_key  TEXT    NOT NULL,
    item_id    TEXT    NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    PRIMARY KEY (library_id, match_key)
);
CREATE INDEX item_aliases_item ON item_aliases (item_id);
