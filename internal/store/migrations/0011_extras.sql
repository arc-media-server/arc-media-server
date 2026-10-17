-- Extras: trailers, featurettes, behind the scenes, deleted scenes... kept beside a movie or show.
CREATE TABLE extras (
    id      TEXT    PRIMARY KEY,
    -- The movie or show it belongs to.
    item_id TEXT    NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    file_id INTEGER NOT NULL UNIQUE REFERENCES media_files (id) ON DELETE CASCADE,
    -- trailer, featurette, behind_the_scenes, deleted_scene, interview, scene, short, other
    kind    TEXT    NOT NULL,
    title   TEXT    NOT NULL
);
CREATE INDEX extras_item ON extras (item_id);
