# Fix Match closes at once; a show's episodes are fetched in the background

From the owner: "When fixing matches, and selecting the correct show, the menu seems to hang.
I'd imagine it's grabbing everything; if that's true this could be done in the background."

## The cause

It was slowness, not a lock or a frontend bug. `PUT /api/items/{id}/match` for a show ran the
whole series sync before answering, while the dialog sat on a spinner:

- TheTVDB: sign-in, the series record, its translation, and every episode order it has
  (aired, DVD, absolute and so on: one paged request per order, two per order in a library whose
  language isn't English);
- TMDB: the show with credits, ratings, artwork, keywords and recommendations, every season
  (20 per request, twice for untranslated episodes) and every episode group;
- then, per episode, three writes (the episode, its metadata, its guest stars and crew), the
  mapping between the two providers' episodes, every order's entries and special placements;
- and it then started a full scan of the whole library to relink the files.

It also refused (409) any show's Fix Match while any library was scanning.

Timed against a local stand-in for both providers answering every request after 200 ms (a real
round trip; real TMDB answers with large `append_to_response` are often slower), no real keys:

| Show | Before: answer | After: answer | After: background task |
|---|---|---|---|
| 8 seasons × 12 episodes, TheTVDB chosen | 2.45 s (11 requests) | 0.61 s (3 requests) | 3.0 s |
| same, TMDB chosen | 2.70 s (12 requests) | 0.21 s (1 request) | 2.9 s |
| 40 seasons × 25 episodes, TheTVDB chosen | 4.34 s | 0.61 s | 5.9 s |
| 40 seasons × 25 episodes, TMDB chosen | 4.35 s | 0.22 s | 5.7 s |
| 8 × 12, every request 500 ms | 5.82 s | 1.51 s (TheTVDB) / 0.52 s (TMDB) | 6.9 s |

With no provider latency at all the database work alone was 0.53 s (96 episodes) and 2.2 s
(1,000 episodes) on this machine; on a busy NAS disk it is several times that. The background
task is a little longer than the old request because it now relinks the show's files itself
(instead of a separate scan of the whole library) and TheTVDB's series record is read twice.

A movie is one TMDB request (0.21 s at 200 ms), so movies stay as they were: matched before
the answer.

## The design (as Plex does it)

Plex closes Fix Match as soon as a result is picked and refreshes the item in the background,
the title updating when it's done. Arc now does the same:

- **Quick part, before answering (`scanner.SetMatch`)**: checks the chosen entry exists and reads
  only its own record (TMDB: one `/tv/{id}` request, new `TMDB.TVSummary`; TheTVDB: the series
  record), then saves the provider ids, title, year, overview, poster and backdrop through
  `store.UpdateMetadata` (locked fields are kept) and marks it matched. The old match's tagline,
  rating, studio and so on are cleared rather than shown against the new show. The other
  provider's id is taken from the chosen record when it has one, so the background part follows
  that link instead of searching the other provider by the old title. The answer is 202 with the
  item and `match_task_id`. If the entry can't be fetched (not found, provider down), nothing
  changes and the dialog shows the error with Retry.
- **Background part**: the existing series refresh (`RefreshSeries`, refactored so a caller can
  hand it its own Activity task) runs under the library's work slot: all seasons, episodes,
  orders, credits and season posters, then relinking the show's files to the new episodes and
  removing orphaned ones. It's listed in Activity as "Matching <title>" ("Matched <title>" when
  done) and can be stopped there. The full library scan after a match is gone: the refresh
  relinks the show's own files, as Refresh metadata does.
- **Busy library**: the match no longer refuses while a scan runs. The task waits for the
  library's current work ("Waiting for the library's current work to finish") and then runs. A
  scan that read the show before the match was saved can't undo it: the chosen ids are put back
  inside the work slot before the refresh.
- **A second Fix Match** of the same show replaces one still waiting (it's dropped from Activity);
  one already running finishes first and the newer one runs after it, so the latest choice
  always wins. A Refresh metadata during a match gets the usual "the library is busy" answer.
- **Failures are visible**: the Activity task shows the error; the show's match explanation (Fix
  Match's "How it was identified") gains a step with the error and "Refresh metadata to try
  again"; and, as for any failed library work, a server problem notice goes to webhooks. The new
  match is kept (the entry exists) and the show stays "not yet fully fetched", so the next scan
  or Refresh metadata completes it.
- **Watch history**: episodes known only from their file names (an unmatched show) are adopted
  by the provider's episodes with the same ids, as in any refresh, so resume points and watched
  state stay.

The dialog closes as soon as the match is accepted. A note at the bottom of the page ("Updating
Arc Show from TMDB…") follows the task through the Activity list the app already polls; when it
finishes, the show's page, seasons, orders and lists are refetched and the note says "Updated …"
for a few seconds. If it failed, the note stays with the error, Try again (the same match again)
and Dismiss. The note lives outside the dialog, so it also works from a poster's ⋯ menu and the
Health page.

## Limits

- A show re-matched from one provider entry to a *different* one gets new episodes; watch history
  on the old (wrong) episodes is kept on them but not moved to the new ones. This was already so.
- If Arc restarts during the background part, the show keeps the new match and its own details,
  and the next scan fetches the rest (it's still marked as not fully fetched).
- The page note is per browser tab; another device sees the change when its pages next refresh.
- Decision for #80: a failed background part keeps the chosen match rather than going back to
  the old one (Plex keeps the match too; going back would undo a choice that was valid).

## Checks

- Go: new tests in `internal/scanner/match_background_test.go` (answer before the episode
  requests are released; the task fills every episode and relinks every file; watch history and
  episode ids survive; a second match, a refresh and a scan during a match leave exactly one set
  of episodes and the latest match; a failure shows in Activity and the match explanation;
  an unknown id changes nothing; movies stay synchronous) and
  `internal/api/match_background_test.go` (202 with `match_task_id` while the episode lists are
  held). A stand-in for both providers, never real keys.
- Browser: `web/e2e/fix-match.spec.ts` (dialog closes while the episode lists are held, note,
  title, episodes and resume point after; a background failure with Try again; a quick-part
  failure with Retry in the dialog). The fixture server can now hold or fail just the episode
  requests or just the quick lookup.
- Results: `go vet ./...` and `go test ./...` pass; `go test -race ./internal/api/...
  ./internal/scanner/...` passes; `npx tsc -b`, `npx tsc -p e2e/tsconfig.json` and
  `npm run build` pass. Full browser suite: 130 of 131 passed. The one failure,
  `home-navigation.spec.ts:45` (row scroll buttons), is an existing flake: it failed 2 of 3
  runs with main's `App.tsx` built in, and 1 of 3 with this change. `fix-match.spec.ts` passed
  9 of 9 (3 tests × 3 runs).
