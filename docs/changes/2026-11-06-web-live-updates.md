# Web: live updates

The web app now refreshes what's on screen when Arc says something changed, instead of only on
reload. Arc's events stream (`/api/v1/events`, docs/EVENTS-API.md) has sent "something changed"
hints since 31 October, and more background work sends them since #336. The web app wasn't
listening.

## What changed

- **One stream per tab** (`web/src/lib/live-updates.ts`, mounted in `App.tsx` once a profile is
  chosen, reopened when the profile changes). The browser's EventSource reconnects by itself and
  resumes from its last event.
- **Each hint marks the matching screens out of date.** A hint names a topic (catalog, library,
  profile, playback, artwork…), never the record. Each topic maps to the query keys it can
  affect (`topicQueries`). Visible screens refetch; the rest refetch when next shown. `resync`
  refreshes everything. Unknown topics are ignored, since the server may add more.
- **Refreshes are spaced out.** During a scan Arc sends hints twice a second for minutes. The
  first refresh comes 1.5 s after a hint, then at most one every 5 s while hints keep coming,
  so a scan shows its progress without the page reloading constantly.
- **The stream closes in a hidden tab** after a minute, since the server allows 32 streams in
  all, and reopens when the tab is shown again. Because hints could have been missed, everything
  visible refreshes then.
- When the server refuses a stream (429, all streams in use, or signed out), it's retried after
  5 s, doubling up to a minute, with a full refresh once reconnected.

## Decisions

- Matches what Plex's web app does (screens update as the server changes) without a setting.
- No change to the 30-second freshness or refocus behaviour: hints are best-effort, so screens
  still refresh as before when hints are missed.

## Checks

- `tools/live-updates.test.mjs` (in `npm run build`): topic mapping, resync and unknown topics;
  a scan-like stream of hints (two a second for 20 s) gives a refresh 1.5 s in, then one every
  5 s or more; a single hint refreshes after 1.5 s; stopping drops pending hints.
- Browser journey `e2e/live-updates.spec.ts`: renaming a movie through the API (as another
  device would) updates its open page, and then Home, without a reload, over one stream. It
  fails with the listener removed.
- The full browser suite was run; results are in the PR.

## Logging

None added. The server already logs stream failures. The browser shows connection retries in
its own developer tools only; nothing is logged per hint.

## Limits

- Images that failed to load before their artwork arrived aren't retried; the queries holding
  their URLs refresh, which is enough where the URL changes.
- Screens that poll already (Activity, Dashboard) keep polling; the hints just make them sooner.
