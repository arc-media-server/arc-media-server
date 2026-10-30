-- Identity is independent of the friendly name, network address and server version.
-- A full database restore intentionally restores this identity too.
INSERT OR IGNORE INTO settings(key,value) VALUES ('server_id',lower(hex(randomblob(16))));
