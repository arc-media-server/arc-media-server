-- Explicit Unmatch stays unmatched until Fix Match is chosen, including after scans/restarts.
ALTER TABLE items ADD COLUMN match_suppressed INTEGER NOT NULL DEFAULT 0 CHECK(match_suppressed IN (0,1));
