# Developer docs: Downloads are merged; attribution ticked

The last two items on #139 (API docs page):

- `docs/CLIENT-INTEGRATION.md` still described downloads as pending PRs awaiting review, and
  linked to a pinned commit of the downloads guide. Downloads (preparation, offline bundles,
  resumable transfer) are merged (#441/#444). The section now says so, keeps the rule to gate on
  capabilities (older servers don't advertise them), says offline progress sync, converted
  downloads and native device acceptance are still missing (#62), and links the guide on main.
  `docs/DOWNLOADS-OFFLINE-API.md`'s introduction is updated the same way.
- The Watch Together developer-docs journey asserted that the download capabilities were absent.
  It now asserts they're supported and allowed for the administrator, with `available` a boolean.
  The guide journey checks the new link.

Also ticked the provider attribution items (#64) in `docs/ALPHA-CHECKLIST.md` and `ROADMAP.md`:
TMDB and TheTVDB credits show on film and show pages and in Settings.

Validation: `go test ./internal/apidoc`, production web build, and all six
`web/e2e/developer-docs.spec.ts` journeys (6 passed, using the machine's Chromium without video).
Documentation and test only; no logging changes.
