ALTER TABLE libraries ADD COLUMN season_label_mode TEXT NOT NULL DEFAULT 'number'
    CHECK (season_label_mode IN ('number', 'name'));
ALTER TABLE items ADD COLUMN season_label_mode TEXT NOT NULL DEFAULT ''
    CHECK (season_label_mode IN ('', 'number', 'name'));
