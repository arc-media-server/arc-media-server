-- A display marker only: tools use ordinary device-session credentials and authorization.
ALTER TABLE sessions ADD COLUMN is_tool INTEGER NOT NULL DEFAULT 0 CHECK (is_tool IN (0, 1));
