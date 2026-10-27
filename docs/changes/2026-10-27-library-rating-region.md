# Age ratings per library (27 October)

Owner request: a library's age ratings should follow a country chosen for that library, as
Plex's per-library Certification Country, not one server-wide Region.

- Add/Edit library → Advanced → Metadata has an "Age ratings" country under Metadata language.
  New libraries start with the server's country (set from the browser at first-run setup).
- Migration 0056 adds libraries.rating_region and fills existing libraries with the server's
  current Region, so nothing changes until a library picks another country.
- Changing a library's country refetches that library's titles (a normal Refresh all metadata
  under the library lease; if the library is busy, the maintenance refresh catches up). Field
  locks still protect an edited rating. Other libraries are untouched.
- The Region row is gone from Settings → Online metadata. The server value remains only as the
  default for new libraries (setup and PATCH /api/settings still accept it); changing it no
  longer refetches every library.
- Parental controls already compare derived ages (12A → 12, PG-13 → 13), so libraries using
  different countries are limited consistently.

Checks: go vet/test (new upgrade and rating-choice tests), tsc app/e2e, production build, and
Playwright maintenance, clarity, ux and journeys specs (78 journeys including a new one that
switches a library from GB to US and sees PG become PG-13 without a manual refresh).
