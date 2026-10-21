# Real-server trial (#137)

Status: not started by these automated checks. Run for 7–14 days on the intended Unraid/NAS
installation, after saving a stopped full data-directory backup and the current immutable image
tag. Do not perform destructive recovery tests on the trial's real media or only copy of data.

## Record the baseline

| Field | Result |
| --- | --- |
| Start/end dates | Pending |
| Image commit/digest | Pending |
| Host OS and hardware | Pending |
| FFmpeg version / GPU driver | Pending |
| Library title/file counts and storage type | Pending |
| Data-directory and database sizes | Pending |
| Resting memory/CPU/network | Pending |
| Conversion location and free space | Pending |
| Maintenance window and enabled background work | Pending |

Record resource measurements from Arc's Dashboard at the start, during ordinary playback,
after playback ends, during overnight work, and at the end. For containers, these read-only
commands provide additional evidence (replace `arc` with the actual container name):

```sh
docker stats --no-stream arc
docker inspect --format '{{.State.StartedAt}} {{.RestartCount}} {{.State.Health.Status}}' arc
docker exec arc ffmpeg -version
docker exec arc sh -c 'du -sk /config; df -k /config'
```

For database and conversion-space measurements use the actual configured paths, including any
conversion folder outside `/config`. Do not copy database files while running and treat that
copy as a verified backup. Built-in backups are consistent database snapshots; stopped complete
directory copies also preserve uploads and certificates.

## Daily record

| Date/time | Activity and clients | Resting/active memory and CPU | DB/temp/free space | Restart/errors | Issue/evidence |
| --- | --- | --- | --- | --- | --- |
| Pending | | | | | |

Use the server normally: add new media, rescan, edit details, change display ordering, switch
profiles, watch and resume, and allow maintenance to finish. Note whether resources return
toward the baseline afterwards. Keep evidence for actual direct play, remux and video conversion
separate, using the player's playback information. Record device/browser versions and media
characteristics; automated Chromium fixtures do not prove TV/casting/HDR/GPU compatibility.

For each failure, record expected/actual behavior, time, commit, device and steps to reproduce.
Redact credentials, invite/session/cast tokens, public addresses and private media paths before
sharing logs. File reproducible defects rather than treating an empty log as proof of stability.

## Completion

- Record the final measurements and any unexplained growth against baseline.
- Resolve or explicitly track errors, unexpected restarts and critical playback/access issues.
- Link the evidence on #137 and update `docs/READINESS.md`; do not close the issue just because
  the requested number of days elapsed.
- Restore/rollback, real-device playback, provider approval and attribution have separate
  acceptance checks. A successful trial alone does not approve public release.
