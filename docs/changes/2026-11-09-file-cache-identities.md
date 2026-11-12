# #350: preview/downloaded-subtitle caches belong to a source, not a row number

Real disposable-store regressions deleted a library, reused its numeric media-file id and then
received the old file's preview info/sheet and downloaded subtitles. Numeric ids remain compatible;
add a private persistent cache_key per file row (migration 0066). New rows receive a fresh random
identity. Content size/time changes rotate it; unchanged scans and technical reanalysis retain it.
Catalog-only renames retaining the row (PR #359) retain it too. It is excluded from API JSON.

Scope preview sheets/info/BIF and downloaded subtitles to id/v2-<source-key>. Checking and serving
use the same revision path, so an old in-flight generation cannot overwrite the new source's assets.
The worker uses its bulk VideoFiles revision snapshot, avoiding a DB lookup per candidate; API
requests perform a bounded lightweight current-key lookup. Existing subtitle extraction/font caches
already include source path/size/time fingerprints and are outside this layout change.

Legacy unverified preview/downloaded layouts are not automatically associated with current rows:
old rowids alone cannot establish ownership. Previews regenerate; old downloaded subtitles remain
on disk but are not offered for playback. They may need downloading again. Do not silently migrate
possibly wrong subtitles. Existing cleanup preserves unknown legacy subtitle ownership while a
numeric row remains; deleted-row artifacts follow its existing orphan policy. No owner cache/data
was inspected or changed during development.

Extend the existing age-guarded, root-confined maintenance cleanup to revision directories. It removes
obsolete/deleted source assets, retains the current revision even for temporarily missing media,
and preserves unknown files/symlinks. No new job, arbitrary directory deletion or media writes.
Post-upgrade backups preserve the source key; restoring pre-upgrade backups generates fresh identities
rather than trusting potentially reused legacy caches. This identifies catalog/source revisions,
not a cryptographic hash of all media bytes.

Tests cover the real reused-id regressions, current/offline versus obsolete cleanup, reanalysis and
content-change revision boundaries, hidden API fields and existing real FFmpeg preview/BIF generation.
Logging reuses preview and cleanup outcome/failure summaries; no new per-request/file records, keys,
raw locations or credentials. Source keys are never logged. Physical deployment remains unverified.
