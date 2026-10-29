-- Shared library Recommended layout; profile watch state still decides each row's content.
CREATE TABLE library_recommended (
    library_id INTEGER PRIMARY KEY REFERENCES libraries(id) ON DELETE CASCADE,
    layout TEXT NOT NULL
) WITHOUT ROWID;
