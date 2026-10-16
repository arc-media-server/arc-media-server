-- TV shows: metadata sync tracking, provider lookups and ordering descriptions.

-- When provider metadata (and, for series, orderings) was last fetched. 0 = never.
ALTER TABLE items ADD COLUMN synced_at INTEGER NOT NULL DEFAULT 0;

CREATE INDEX items_parent_tvdb ON items (parent_id, tvdb_id);
CREATE INDEX items_parent_tmdb ON items (parent_id, tmdb_id);

-- Short label for the kind of ordering, e.g. "Story Arc" or "DVD" for TMDB episode groups.
ALTER TABLE orderings ADD COLUMN description TEXT NOT NULL DEFAULT '';
