# Database and cache upkeep (#203)

Settings → Maintenance & diagnostics lists **Optimize database** and **Clean up cached files**
with an enable switch, live status, Run now and a saved last result. Both default On, including
existing installations. Scheduled work runs weekly in the existing server-local maintenance
window. Run now bypasses the window/cadence, while playback and foreground library work still
have priority. Repeated requests coalesce, and optional workers share the existing FIFO slot.

Successful completion, last attempt, result and failure are stored in the existing settings
table, surviving restart; no migration is required. Failed/paused work retries no more than
hourly during maintenance, or explicitly with Run now. Unfinished startup markers become a
visible interrupted result. Settings/status read or write failures are reported in Activity,
not treated as completed. Current optional work is cancelled when it becomes ineligible.

## Optimize database

- Reserve the existing SQLite writer connection rather than replacing the live database.
  Independent WAL readers remain usable; other writes queue while upkeep holds that connection.
- Run a full integrity check before work. Any non-ok row or SQL failure aborts and is reported.
- Run bounded `PRAGMA optimize=0x10002` to update query statistics as SQLite judges necessary.
- Compact with VACUUM only when at least 20% of pages and 16 MiB are unused, the task remains
  eligible, and both database and OS temporary volumes report sufficient free space.
- Require two database sizes plus 32 MiB headroom on each volume. Unknown/low space skips
  compaction and explains why, while statistics/integrity work can still complete.
- Check integrity again. After VACUUM, attempt WAL checkpoint/truncation; active readers can
  defer the final disk reclamation. Report this rather than claiming space was already freed.

The before/after size includes the main database and WAL. Statistics can grow a small database;
optimization does not imply every run shrinks it. Compaction is conditional, not a corruption
repair tool or a substitute for backups. No measured lookup-speed improvement is claimed.
SQLite cancellation/disk-error handling remains transactional; space checks cannot reserve
capacity against other processes. Physical low-space, storage-failure and power-loss recovery
remain owner-server checks. Linux/Windows have native available-space probes; other platforms
skip VACUUM with an explicit unavailable-space-check reason.

## Clean up cached files

The task walks only these fixed application cache roots, using rooted filesystem operations:

| Root | Identified artifacts | Retained when |
| --- | --- | --- |
| artwork | URL-hash images / partial downloads in their hash shard | Any item/season/person/collection still references that source |
| trickplay | Numeric file-ID folders, sprite JPEGs, info.json and known temporary folders | File record still exists |
| videothumbs | Numeric file-ID JPEGs | File record still exists |
| subtitles | File-ID cue JSON and downloaded/file-ID subtitle tracks | File record still exists |
| optimized | Recognized generated MP4/partial names | File record or registered artifact still exists |

Missing/offline and unmatched media records count as references. Cleanup does not scan or stat
library media to decide ownership. Known empty generated containers are removed; unknown names,
non-regular files and nested symlinks are retained. A symlinked cache root is refused. Files
newer than one hour are kept so recently generated/in-flight work can finish. Errors stop the
pass and retain a failure with any partial count. Reclaimed bytes sum removed file lengths,
not an allocation-level disk measurement.

Backups, uploads (original user artwork), media, certificates, logs, ratings files and transcode
sessions are excluded. Their existing managers/retention policies are unchanged. Provider
preview metadata is already bounded in memory and has no on-disk bundle to remove.

Old flat subtitle cue hashes have no saved owner. They are **kept**, reported in the result and
adopted into the owning file-ID folder when used successfully. New cue entries use file-ID
folders immediately. This avoids guessing which historical entries belong to retained offline
files. It also means the first cleanup cannot remove every unidentified historical cue cache.

## Plex reference and differences

Official references reviewed for this implementation:

- [Scheduled Tasks](https://support.plex.tv/articles/201553286-scheduled-tasks/): weekly database
  optimization and bundle/cache maintenance, controlled in server maintenance settings.
- [Troubleshooting](https://support.plex.tv/articles/226836308-help/): manual Optimize Database
  and Clean Bundles, with completion feedback.

Arc puts manual and scheduled controls in one Maintenance list, as requested in #202/#203.
It uses its own SQLite/cache architecture; no Plex internal implementation or performance is
inferred. Arc's one-hour grace applies only to identified unreferenced generated artifacts;
it does not evict referenced images on an age-only policy. Unknown legacy subtitle entries and
user uploads are preserved. These conservative differences are recorded on GitHub #80.

## Verification

READINESS and the #203 PR handoff record actual counts. Isolated checks cover a real VACUUM,
low/unknown-space refusal, corrupt integrity results, cancelled/busy admission, weekly cadence,
manual coalescing, persisted success/failure, restart/interruption, default-On/explicit-Off,
cache ownership including missing files, unknown files, symlinks and legacy subtitle adoption.
Browser checks use generated clips/local providers/pinned Chromium and exercise manual work,
persisted last results, settings, failures/Retry and narrow layouts. Windows cross-compilation
is distinct from executing Windows tests. Hosted verification runs on the PR; the owner asked
agents to stop after PR creation rather than wait for those checks.
