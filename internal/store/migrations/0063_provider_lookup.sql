-- Provider IDs are not unique: libraries, editions and episode providers can overlap.
CREATE INDEX items_lookup_tmdb ON items(tmdb_id,kind,library_id,id);
CREATE INDEX items_lookup_tvdb ON items(tvdb_id,kind,library_id,id);
CREATE INDEX items_lookup_imdb ON items(imdb_id COLLATE NOCASE,kind,library_id,id);
