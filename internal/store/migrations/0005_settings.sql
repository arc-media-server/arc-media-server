-- Small server-wide flags and values, e.g. whether the libraries from ams.yaml have been imported.
CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
