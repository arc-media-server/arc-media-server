# Backups and recovery

Arc's built-in backups are **database snapshots**. For full server recovery, keep a stopped
copy of the Arc data directory. With PostgreSQL, also keep a current backup of the external
database. Preserve source media separately.

## What to preserve

| Data | Built-in database backup | Stopped data directory |
| --- | --- | --- |
| Matches, library paths, metadata edits/locks and episode orders | Yes | SQLite only |
| Profiles, credentials/settings, watch history and resume | Yes | SQLite only |
| Uploaded artwork in uploads/ | No | Yes |
| Certificates/keys stored in certs/ | No | Yes |
| Optimized videos, downloaded subtitles, cached artwork, previews and logs in the data directory | No | Yes |
| Media, external certificates, custom temporary paths and deployment environment/configuration outside that directory | No | Preserve separately |

Database snapshots contain sensitive configuration and access information; keep them and
full backups in storage you control. Uploaded artwork cannot be reconstructed from SQLite.
Regenerating optional assets or fetching them again depends on the source, provider and
policy being available; keep wanted assets rather than assuming every cache can be rebuilt.
Optimized copies require their files or explicit regeneration.

## SQLite full backup

1. Record the running Arc version or immutable Docker commit tag and retain that image/binary.
2. Stop Arc cleanly (stop its container/service/process) and wait for it to exit.
3. Copy the **entire data directory** to separate storage, preserving its contents and
   permissions. In Docker this is the host folder/volume mounted at `/config`; ordinary
   installations use their configured `-data`/`AMS_DATA` directory. Include any SQLite
   companion files still present; do not copy only a live ams.db or delete its WAL files.
4. Keep media, external certificate/key files, deployment settings/environment and any
   other externally configured locations separately. Preserve the paths Arc expects.
5. Keep the backup intact and check recovery using a working copy, not the only backup.

Back Up Now is useful while Arc runs because it takes a consistent SQLite snapshot. It
is not a live full-directory snapshot facility. Automatic and safety database backups
follow the existing policy shown in Backups; full copies are an operator action.

## SQLite recovery or rollback

1. Stop the Arc instance being recovered. Preserve its current data separately before
   replacing anything, so a failed recovery can be undone.
2. Restore the complete saved directory into a separate empty writable location, preserving
   appropriate ownership. Use the matching saved version/image and expected media/external
   paths. Never run two instances against the same data directory.
3. Start the recovery instance and verify sign-in/profiles, settings, library paths, metadata
   edits/locks, episode ordering, watch history/resume and an uploaded poster. Check that
   media is available and remains read-only; missing drives are not empty collections.
4. Verify external certificates, networking and storage separately before normal use.
   A cloud fixture does not establish your NAS/device/network acceptance.

Older executables reject newer schemas. To roll back an upgrade, pair the previous
image/binary with its compatible pre-upgrade full snapshot, rather than pointing it at
the upgraded database. Arc's database restore validates snapshots, preserves a safety
copy first and migrates supported older snapshots for the **current** executable.

## PostgreSQL setup and recovery

**First-time setup.** Open Arc from the host itself or forward the host port over SSH. For
example, run `ssh -L 8099:127.0.0.1:8099 user@arc-host` on your computer and open
`http://127.0.0.1:8099` there. The SSH tunnel protects the browser-to-Arc hop that carries
the PostgreSQL password. Leave certificate and hostname verification enabled for the
separate Arc-to-PostgreSQL connection. For a private issuing CA, mount its public PEM
read-only and set `PGSSLROOTCERT` to the in-container path. The Arc user must be able to
read it.

**Full backup.** Record Arc's image tag, stop Arc, then take a PostgreSQL backup or provider
snapshot and copy the stopped `/config` directory. Preserve the media paths and private CA
separately. Arc's `.pg.zip` archives from **Back Up Now** are verified database snapshots
while Arc is running. If you use one of those for full recovery, record its time and account
for any later database or file changes. The copied `database.json` contains credentials and
connection settings. Protect it and the database backup.

**Restore an Arc archive.** Stop Arc, preserve the original data directory, and create a
separate, empty PostgreSQL database owned by the recovery account. Use an Arc image
with the recovery command that recognises the archived schema. Set
`ARC_POSTGRES_RECOVERY_DSN` securely in your shell to the new
database connection. With a private CA, run:

```sh
docker run --rm --network "$POSTGRES_NETWORK" --user "$(id -u):$(id -g)" \
  -e ARC_POSTGRES_RECOVERY_DSN -e PGSSLROOTCERT=/certs/postgres-ca.pem \
  -v "$ARC_ARCHIVE:/recovery/archive.pg.zip:ro" \
  -v "$POSTGRES_CA:/certs/postgres-ca.pem:ro" \
  --entrypoint /usr/local/bin/arc-postgres-recovery "$ARC_IMAGE" /recovery/archive.pg.zip
```

Use absolute host paths in `ARC_ARCHIVE` and `POSTGRES_CA`, the Docker network that
can reach PostgreSQL in `POSTGRES_NETWORK`, and the recorded image in `ARC_IMAGE`.
With the system trust store, omit the CA mount and `PGSSLROOTCERT` option. Recovery
defaults to `sslmode=verify-full`. An explicit `sslmode=disable` is available for a
trusted private network. For a PostgreSQL-native backup or provider snapshot, use its
own restore procedure to populate the separate database.

**Switch Arc to the restored database.** Work from a separate, writable copy of
`/config`. While Arc is stopped, edit the saved PostgreSQL `dsn` in that copy's
`database.json` with a private editor, leaving `engine` and `installation_id` intact
and preserving permissions. Use the restored database's hostname and URL-encode any
special characters in the login. Include `sslmode=verify-full` in the saved connection
unless you deliberately chose the private-network override. Start Arc with the copied directory and matching
image, then check profiles, libraries, watch history and artwork before normal use.
To reverse the cutover, stop Arc and reconnect the original directory, database and
matching image. Writes made after the cutover stay on the restored database.

## Database restore has a narrower boundary

The SQLite Backups page restores database state only. Uploaded artwork, certificates and generated
files are not rolled back. For example, replacing a poster can remove the previous upload;
restoring the old database restores its reference but cannot recreate the old image file.
The database safety copy makes database restore undoable, not a full-filesystem rollback.
Use the full copy for whole-server recovery or re-upload the intended artwork if needed.

## Verified scope and remaining acceptance (#136)

The fresh isolated recovery rehearsal closes the store before copying, opens an independent
restored directory and checks actual uploaded PNG bytes through the authenticated API,
settings, locked title/poster, stable identity, history/resume and opaque certificate-fixture
bytes. It also proves DB-only restore cannot recreate a replaced upload. This is not a
real certificate/TLS, physical disk, power-loss or owner-server acceptance test.

Existing automated fixtures cover unavailable/reattached roots, interrupted and killed
scanner processes, provider failures, backup-directory obstruction, SQLite capacity errors,
old-schema upgrades/restore, invalid snapshots, conversion-storage obstruction, stopped
FFmpeg and restart cleanup. Container acceptance separately exercises stopped-folder
upgrade/rollback copies with root/non-root owners. Fresh results are in READINESS and #136;
earlier hosted results remain historical until rerun for the current commit.

Still required on owner-controlled disposable copies: actual filesystem ENOSPC/permission
behavior for configured temp/backup storage, abrupt whole-server loss during active playback/
conversion, real storage/mount recovery, external TLS/environment restoration and a complete
owner deployment recovery rehearsal. #136 remains open for those checks. Do not use the
owner's only database, real media or live system for destructive failure simulations.
