# Downloads backend groundwork (#62)

Define the scope and phased backend/client contract before runtime work. Current main's ranged
streaming, optimized copies, auth/device sessions, watch state and SQLite lifecycle are useful
foundations but do not implement offline downloads. The existing stream route also registers
active playback, so download transfer must use a separate path.

- `docs/plans/DOWNLOADS.md` inventories reuse points, ownership/permission defaults, source
  integrity, durable lifecycle, background bounds, cleanup, sync conflicts and phased acceptance.
- `docs/DOWNLOADS-API-DRAFT.md` gives proposed original-download/manifest/asset/event contracts,
  valid illustrative JSON and a client handoff. It explicitly says no endpoints/capabilities are
  implemented and that the generated contract of future handlers takes precedence.
- First usable scope: explicit original movie/episode downloads plus safe reconnect sync.
  Complete compatible conversions follow later; automatic season rules and detailed offline
  history are deferred. Already copied bytes cannot be remotely recalled.
- Target SQLite per the owner's deferral of external databases. Runtime PRs follow reviewed #418
  integration; no merges of that draft, PostgreSQL or client-repository work is performed.
- ROADMAP/CLAUDE point to the proposal without claiming feature/release/device completion.

Official Plex overview/FAQ fetches both returned 403 on 11 November; current parity/defaults are
not asserted. This is a source-backed Arc proposal, not a copy of undocumented Plex internals.

Validation: illustrative JSON syntax, local Markdown links, source reuse references and diff check.
No runtime/source/dependency/schema/API-snapshot changes, so no Go/browser suite rerun. No new
logging; future implementation must follow bounded outcome/error diagnostics and privacy rules.
No owner data, keys, media, Unraid, persistent server, native device or FFmpeg conversion used.
