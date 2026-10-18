-- Apps sign in with a token (sent as "Authorization: Bearer") instead of a browser cookie. Their
-- sessions carry the name the app gave itself, e.g. "Living Room Apple TV", for Settings > Devices.
ALTER TABLE sessions ADD COLUMN device TEXT NOT NULL DEFAULT '';
