-- A season poster the user chose (a TMDB path or "upload:<file>"); it wins over the folder's and
-- the provider's, and refreshes keep it.
ALTER TABLE season_art ADD COLUMN chosen_path TEXT NOT NULL DEFAULT '';
