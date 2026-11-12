ALTER TABLE downloads ADD COLUMN snapshot TEXT NOT NULL DEFAULT '';
-- Earlier ready records have no immutable metadata/asset snapshot. Explicit reprepare
-- is required; do not invent an offline bundle by rereading mutable metadata in GET.
UPDATE downloads SET state='failed',reason_code='bundle_unavailable',sha256='',source_mtime_ns=0;
