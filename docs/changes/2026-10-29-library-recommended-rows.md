# More rows on a library's Recommended tab

The owner asked for a library's Recommended tab to have Plex's rows. Before this it had only
"Because you watched", "Top rated you haven't seen" and "More <genre>", so a profile with little
history saw a single row.

What changed:
- **`GET /api/libraries/{id}/recommended` returns these rows, in this order**, each left out when
  it has nothing:
  1. **Continue Watching**: this library only. It's Home's list (what the profile is part-way
     through, and in a TV library the next episode of shows it's watching), with cards that play.
  2. **Recently Released** (newest release date first).
  3. **Recently Added in <library>**.
  4. **Start Watching** (TV libraries): shows with no episode watched or part-watched, in an order
     that changes daily.
  5. **Because you watched <title>** for up to three recent titles (unchanged).
  6. **Top Rated Movies / Top Rated Shows**, watched titles included.
  7. **Recently Watched** (movies), or **Recently Watched Episodes** in a TV library: episode cards
     numbered in the show's episode order, like Home.
  8. **Rediscover**: titles the profile finished and hasn't played for six months, in an order
     that changes daily.
  9. **More in <genre>** for the two favourite genres (unchanged apart from the title).
  10. **Top Movies/Shows in <genre>**: a genre with at least five titles here, a different one each
      day, not one of the favourites above, by rating, watched included.
  11. **Top Movies/Shows by <director>** or **with <actor>**: someone credited on at least three
      titles here, by rating. One credited on what the profile watched most recently is preferred
      (rotating daily among the three most recent); otherwise one of the ten with the most titles.
      The row title opens the person's page.
  12. **Top Unwatched** (was "Top rated you haven't seen").
- **Rows are titled in the app.** The server sends each row's `type` and its parameters (`name`,
  `genre`, `person_id`, `role`) instead of an English title, and the web app titles them with
  `t()` from `library.json` (`recommendedRows.*`). Old titles built on the server broke the rule
  that UI text is translatable.
- **"See all" links** where the library has the same list: Recently Released → `?sort=release`,
  Recently Added → `?sort=added`, Top Rated → `?sort=rating`, Recently Watched → `?sort=watched`,
  More in → `?genre=…&unwatched=1`, Top in genre → `?genre=…&sort=rating`, Top Unwatched →
  `?sort=rating&unwatched=1`, person rows → `/person/{id}`. Continue Watching, Start Watching,
  Because you watched and Rediscover have no library view of their own, so they aren't links.
- **Empty state**: now that rows don't need history, the tab is only empty when the library has
  nothing the profile can watch, so it says "Recommendations appear here once this library has
  titles you can watch."
- Removing something from Continue Watching refreshes the Recommended tab too.

How it's built:
- Store: `ItemQuery` gains `Unstarted`, `Played`, `PlayedBefore`, `PersonID` and `PersonRole`
  (all through `itemFilter`, so access, age limits and hidden labels apply as everywhere else);
  new `LibraryGenres` and `FeaturedPeople` queries apply `acc.Where` before counting.
  `RecentlyWatchedSeries` now takes the profile's access and filters before its limit, so Up Next
  for one library isn't crowded out by other libraries' shows.
- API: `writeHome` is split so `homeDTOs` (episode numbering, season posters, watch state) is
  shared by Home and the Recommended rows; `inProgress` and `upNext` take the access to use, and
  the Recommended tab passes the profile's access narrowed to the library.
- One request, at most 20 titles a row. Tests: `internal/store/recommended_test.go`,
  `internal/api/recommended_test.go`, browser journey `web/e2e/recommended.spec.ts`.

Decisions (defaults, easy to change; for #80):
- **Start Watching uses the daily shuffle**, not newest first: for a profile that hasn't started
  anything it would otherwise repeat Recently Added.
- **Rediscover counts a show** when an episode was finished and nothing of it was played in the
  last six months; the show needn't be fully watched. Titles marked watched with no play date
  aren't included.
- **Person rows use lead actors** (the first ten credited) and directors. TV shows rarely have a
  director at series level, so TV libraries mostly get "with <actor>".
- Rows aren't de-duplicated against each other (a title can be in Recently Added and Top Rated),
  as in Plex.

Limits and follow-ups:
- The owner's possible extras aren't in this change: **Short on time** (runtime filter), **From
  your Watchlist** (this library's watchlist titles) and **Critics' picks** (sort `critics`).
- The genre and person of the day are chosen per server day (UTC), not per profile.

Checks (29 October): `go vet ./...` and `go test ./...` pass; `npx tsc -b`,
`npx tsc -p e2e/tsconfig.json` and `npm run build` pass; `e2e/recommended.spec.ts` 2 passed; full
browser suite 130 passed (17.2 min).
