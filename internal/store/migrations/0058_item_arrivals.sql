-- Metadata may precede a file by months. Remember first availability independently.
CREATE TABLE item_arrivals (
 item_id TEXT PRIMARY KEY REFERENCES items(id) ON DELETE CASCADE,
 added_at INTEGER NOT NULL,
 sequence INTEGER NOT NULL
);
CREATE INDEX item_arrivals_sequence ON item_arrivals(sequence);
CREATE TABLE arrival_clock (id INTEGER PRIMARY KEY CHECK(id=1), sequence INTEGER NOT NULL);
INSERT INTO arrival_clock VALUES(1,0);

-- Historical availability cannot be reconstructed. Preserve legacy dates for
-- previously linked (including temporarily missing) items and their shows.
INSERT INTO item_arrivals(item_id,added_at,sequence)
 SELECT id,added_at,ROW_NUMBER() OVER (ORDER BY added_at,rowid) FROM items
 WHERE EXISTS(SELECT 1 FROM item_files f WHERE f.item_id=items.id)
 OR (kind='series' AND EXISTS(SELECT 1 FROM items e JOIN item_files f ON f.item_id=e.id WHERE e.parent_id=items.id));
UPDATE arrival_clock SET sequence=COALESCE((SELECT MAX(sequence) FROM item_arrivals),0);
CREATE INDEX items_parent_added ON items(parent_id,added_at DESC);
