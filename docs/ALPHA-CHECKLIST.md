# Web and Server alpha acceptance (#138)

This is the candidate checklist, not an alpha declaration. Private owner image publication
continues under existing gates. No public release is performed by this review.

## Evidence required for a specific candidate

Record candidate commit, immutable image tag/digest, date, platform/device and the evidence
link for each result. Earlier green runs are historical evidence; unknown is not passed.

- [ ] Candidate Linux/Windows build, TypeScript, Go/vet/race and pinned-browser suite pass.
- [ ] Candidate container acceptance covers clean setup, FFmpeg/FFprobe, health, root and
  non-root ownership, read-only media, persisted settings/logs/uploads, restart/recreation,
  database restore and compatible stopped-folder upgrade/rollback.
- [ ] Reconcile #123 with actual image platforms and documented container behavior. The
  image runs Arc as PUID:PGID (default 1000:1000) and also accepts Docker --user UID:GID;
  do not claim platform or GPU support solely because an image builds.
- [ ] Review unresolved critical access/data-loss defects for the candidate and retain
  reproduction/closure evidence. Test passing alone does not prove absence of defects.
- [ ] #134: representative real-library scanning preserves identity, edits, orders and history.
- [ ] #135/#192: actual browser/device playback/casting results record selected versions/tracks,
  source versus output formats, seeking, reconnect/revocation, failures and receiver limits.
- [ ] #136: complete recovery/storage/mount/external-config rehearsal on disposable deployment
  copies; follow RECOVERY.md. A SQLite backup alone cannot restore uploaded artwork.
- [ ] #137: completed 7–14 day server trial with resource observations and outstanding problems.
- [ ] #64: required provider attribution is visible in its agreed UI locations.
- [ ] #65: provider distribution/access model is confirmed; no owner key is embedded or shipped.
- [ ] Installation, networking, version/rollback and recovery documentation match the candidate;
  test external configuration and hardware separately where claimed.

#123/#64/#65 and #134–#137 were read and remain open at the 25 October review baseline.
Candidate CI/container results are recorded on #138/#162 after the current run completes;
this document deliberately does not tick physical/legal/distribution gates from cloud tests.
Native/offline apps remain outside this Web/Server checklist. Real HDR/Atmos-output evidence
is a device requirement where claimed, not established by rich source-format labels.
