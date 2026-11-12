# Developer documentation corrections

The documentation review found a broken guide link in the served API viewer, profile schemas
that rejected valid requests/responses, incomplete first-client steps, and outdated handoffs.
This change addresses all eight findings without changing authentication, profile access or
playback behavior.

- `/docs/` links to a bundled `/docs/client-integration.html`, rendered from the canonical
  `CLIENT-INTEGRATION.md` through the existing Scalar dependency. Shared appearance stays in one
  place. The core guide works offline; specialist links open repository documentation.
- The walkthrough covers device-token versus cookie sign-in, same-token profile selection,
  browsing, TV linking, preview/start/progress/seek/stop, errors, identifier types and nullable
  profile fields. Named JSON examples are exercised against disposable local fixtures.
- Profile schemas describe library IDs as integer-array or null (all libraries), with an empty
  array meaning none. Excluded labels may also be null. Create requires a name; PATCH fields are
  optional. Generator metadata and supplementation express existing behavior, not new wire rules.
- Contract generation protects each nullable alternative as well as the union. Every existing
  v1 snapshot entry is retained; regression tests cover removal/narrowing and array item types.
- Feature guides distinguish implemented invitations/queues/identity from historical handoffs.
  Dolby Vision base-layer fallback and per-profile SSE limits are explicitly pending #418;
  PostgreSQL #337/#339 remains separate pending work. No deployment/device acceptance is claimed.
- Source prerequisites specify Node 24 LTS or 22.12+, locked npm installs, the tested Jellyfin
  FFmpeg generation, and accurate Bash labels.
- Focused tests cover served navigation, profile schemas/minimal requests, canonical JSON and
  real generated-media playback plans. The existing unused-parameter TS6133 helper fix from #417
  is included so current main's browser test TypeScript can compile; no other review fixes are
  imported from #418.

Validation and limitations are recorded in the PR. Tests use fresh temporary stores, generated
clips and local provider fixtures. Native SDK compilation, physical devices, HDR/audio output,
production deployment and sustained-use acceptance remain separate. No additional runtime
logging is needed: this changes documentation, schema generation and tests; existing request
failure logging and profile permissions are unchanged.

Related: #139. Independent of the unmerged #350 review integration in #418.
