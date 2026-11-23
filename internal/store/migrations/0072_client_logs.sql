CREATE TABLE client_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_hash TEXT NOT NULL REFERENCES sessions(token_hash) ON DELETE CASCADE,
    received_at INTEGER NOT NULL,
    platform TEXT NOT NULL,
    entries TEXT NOT NULL
);
CREATE INDEX client_logs_session ON client_logs(session_hash, id DESC);
CREATE INDEX client_logs_received ON client_logs(received_at);
