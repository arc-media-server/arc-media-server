-- Candidate lookup only; the scanner also checks path absence, naming and uniqueness.
CREATE INDEX media_files_rename ON media_files(library_id, size, mtime);
