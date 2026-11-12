# Complete the developer task walkthroughs (#139)

Follow-up to pending PR #419; that PR already fixes served documentation, schemas and the core
sign-in/browse/playback examples. Keep its changes separate. This branch adds the remaining
client tasks to the same bundled CLIENT-INTEGRATION guide, without changing API behavior:

- Episode orders: distinguish show provider, file numbering and display sequence; discover real
  ordering IDs, preview without saving, render unplaced/unavailable episodes, save/reset an
  admin-only server-wide choice and use returned episode IDs/Next rather than displayed numbers.
  Reset input is JSON null; display_choice_id in the response uses integer zero for inheritance.
- TV linking: name the authenticated approval endpoint and prove one-time token collection and
  profile selection using the same issued device token.
- Watch Together: capability-gated room creation/join/recovery, device/profile ownership, ordinary
  playback responsibilities, heartbeat generation versus command revision, stale-action recovery
  and leave. Full timing/privacy/invitation rules remain in the specialist contract.
- Downloads: explicitly pending #421/#431–#433, unavailable on this main baseline. Do not show
  offline controls from preparation support alone. Explain the capability gate, manifest/revision,
  verified original-byte transfer and separate future sync/conversion scopes. Link to the exact
  reviewed D3 handoff commit, not a nonexistent main document or an implied deployed capability.

Focused journeys exercise the actual documentation JSON, server responses and role boundaries
with generated media, temporary stores and local providers. The first new order check exposed
an incorrect documentation assumption (null instead of zero in the response); both text and
check were corrected. Actual final checks and retained initial failure are recorded in the PR.

The core additions are bundled/offline. Specialist repository links still need network and
repository access, and can describe another server version. No endpoint/schema/contract change,
new dependency, owner data, native-client/device acceptance or persistent Arc server. Full
multi-device synchronization, downloads transfer and native SDK acceptance are not established by
this documentation check. Keep #139 open until its parent/follow-up merge and the combined docs
are checked; public docs hosting remains deferred under the owner's existing decision.

Logging: documentation and fixture checks add no server operation or log volume. Existing
request-failure diagnostics remain responsible; tokens, private invite codes and URLs are never
added to logging. No extra runtime logs are needed.
