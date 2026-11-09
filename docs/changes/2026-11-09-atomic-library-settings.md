# #350: reject or commit the whole library settings save

Reproduced mixed-field PATCH requests changing thumbnails/skip settings before a later invalid
control, empty name or duplicate name rejected the request. Validate independent controls before
any mutation. Share scanner admission with scan/refresh work over a fresh read and save, then
commit core fields/paths, all scalar settings and metadata invalidation in one transaction.
Keep optional controls patch-like by rereading after admission, so an earlier API save isn't lost.

Only after commit, wake workers, forget disabled thumbnail caches, update folder watching and
request the same discovery/refresh operations as before. Validate folder availability only when
core fields are edited, so an ordinary control remains editable with an offline folder. The short
settings reservation is not a scan job: it creates no scan status/history or scan-complete/failure
notification. Pending watcher requests resume when it releases. No locks held across network calls.

Tests first reproduced partial invalid/duplicate saves. Added busy scalar-request admission,
scan exclusion during save, no false scan outcome, and an actual SQLite trigger failure after
core/path updates proving transaction rollback also retains file rows. Existing folder-removal
semantics are shared, not changed here; root-move identity is a separate Low review item.

No API fields/UI changes. Failures use the existing request boundary; invalid/busy input is normal
400/409 validation, with no new info/per-item logs, keys/tokens/bodies/raw URLs or SQL arguments.
Post-commit cache work is not attempted on rejected saves. No owner settings/media/systems touched.
