-- How collections show in a library's grid, as Plex's "Collection mode": a library default
-- (collection_grid: '' the titles only, as before; 'collection' the collection instead of its titles;
-- 'both'), and each collection's own choice (grid_mode: '' follows the library, or 'items',
-- 'collection', 'both').
ALTER TABLE libraries ADD COLUMN collection_grid TEXT NOT NULL DEFAULT '';
ALTER TABLE collections ADD COLUMN grid_mode TEXT NOT NULL DEFAULT '';
