# Empty trash

Empty trash removes database records for missing files, and movies, episodes and shows whose
files are all missing, including their watch history. It never deletes or changes media files.
Titles with any present file remain. This is distinct from scanning and refreshing metadata.

## Manual cleanup

An administrator can use **Manage libraries → library actions → Empty trash…**, or the
existing **Library Health → Missing files → Remove All…**. Both use the same confirmation,
which names the library, explains history loss and warns about disconnected drives.
A safety database backup must succeed before any deletion. Busy libraries return a conflict;
cleanup reserves the same exclusive slot as discovery and metadata work for the entire backup
and deletion. Failures leave the dialog open for retry. No global unconfirmed Run now exists.

## Optional automatic cleanup

**Settings → Maintenance → Empty trash automatically** is **Off by default**, including on
upgrades. Enabling it requires confirmation. Turning it off saves immediately. This differs
from Plex's default intentionally: retaining history on disconnected drives is more valuable
than silently removing entries. Decision recorded on GitHub #80; implementation tracked by #204.
The opt-in EmptyTrashControl is integrated into the task list merged by #202. Its manual
action is Choose library, leading to the library-specific confirmation; other tasks retain Run now.

When enabled, the worker checks once a minute, only during the server-local maintenance window
and while playback/discovery/foreground library work is quiet. It is independent of whether
metadata refresh or ordinary automatic backups are enabled. Safety backups remain mandatory.

A library is eligible only after a successful, complete discovery scan in this server lifetime:

- Every root and walked directory was reachable; file-recording failures also prevent eligibility.
- Partial, failed, cancelled or subsequently stopped scans invalidate earlier eligibility.
- The configured roots must still match the completed scan, in the same order.
- Every root must be readable and contain a readable, known present regular file with its saved
  size. A readable but empty mount directory is insufficient. Legitimately empty roots therefore
  require manual review/confirmation instead of automatic cleanup.
- Files marked missing must still be absent. If a file returns, scan again to reconcile it.
- Scheduling, cancellation, roots and missing-file absence are checked again after backup,
  immediately before the store's transactional deletion.

Restarting Arc discards eligibility; a fresh complete scan is required. Successful cleanup
consumes eligibility, including a pass with nothing missing. No database migration or media
marker files are needed. Watch history and manual edits are otherwise preserved.

Activity records successful removals with library, title/file counts and a bounded sample of
up to five title names. Backup/deletion failures appear in Activity; unsafe automatic skips
leave records intact. Counts concern database records, not deleted files. Existing Activity
retention and safety-backup retention still apply; this is not a permanent audit archive.

## Validation and limits

Focused tests cover the real manual API backup and its pre-deletion contents, access controls,
backup failure, cancellation, lease exclusion, partial/cancelled scans, restart, root changes,
multiple roots, an empty mount point, roots disappearing during backup, returned files and
maintenance/playback gating. Browser journeys cover confirmation/Cancel/Retry, persisted opt-in,
phone geometry, actual generated-fixture deletion, backup presence and Activity results.
See READINESS for actual results, including any corrected test assertions.

All local checks use temporary databases and generated/synthetic media. Actual owner-server
mount behavior, unplugging storage and physical power-loss recovery remain owner checks.
Filesystem availability can change after its final check; checks and the database transaction
cannot make a remote filesystem atomic. A saved readable anchor is conservative evidence of
availability, not verification of a particular physical disk. Leave automation Off when unsure.
