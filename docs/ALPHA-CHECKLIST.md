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
- [x] #64: required provider attribution is visible in its agreed UI locations (film and show pages, and Credits in Settings).
- [ ] #65: provider distribution/access model is confirmed; no owner key is embedded or shipped.
- [ ] Installation, networking, version/rollback and recovery documentation match the candidate;
  test external configuration and hardware separately where claimed.

## Before any repository is made public (owner, 13 November)

Applies to every Arc repository (server, arc-android, arc-apple, arc-roku, arc-tv, arc-kodi), once,
just before it goes public; rewriting history is a deliberate one-off exception to "never force push".

- [ ] Pause work: no agent sessions or Codex running; merge or close every open PR.
- [ ] Scan the full history for secrets (API keys, tokens, passwords, signing material), personal
  paths, names or emails, and files taken from real media or settings; remove any from history.
- [ ] Remove agent session links (`Claude-Session:` lines and other chat URLs) from commit messages
  with `git filter-repo --message-callback`, and from PR descriptions.
- [ ] Force-push the rewritten history once; re-clone everywhere (owner PCs, agent environments,
  Codex); rebuild the container image from the new `main`; check links to old commit hashes (issues,
  ROADMAP, change notes) still resolve or are updated.
- [ ] Only then change the repository's visibility.

#123/#64/#65 and #134–#137 were read and remain open at the 25 October review baseline.
Candidate CI/container results are recorded on #138/#162 after the current run completes;
this document deliberately does not tick physical/legal/distribution gates from cloud tests.
Native/offline apps remain outside this Web/Server checklist. Real HDR/Atmos-output evidence
is a device requirement where claimed, not established by rich source-format labels.
