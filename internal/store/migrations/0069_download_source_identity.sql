ALTER TABLE downloads ADD COLUMN source_cache_key TEXT NOT NULL DEFAULT '';
-- Historical preparation cannot prove a persistent source identity. Keep its bookkeeping,
-- but require an explicit new preparation rather than infer identity from reusable row IDs.
UPDATE downloads SET state='failed',reason_code='source_changed',sha256='',source_mtime_ns=0;
