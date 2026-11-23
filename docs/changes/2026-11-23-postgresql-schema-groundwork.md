# PostgreSQL schema groundwork

[#308](https://github.com/arc-media-server/arc-media-server/issues/308)

Add a developer tool that generates a fresh PostgreSQL schema from all current SQLite migrations.
Reuse the schema conversion from #337 and keep it outside the server binary. The generator covers
columns, defaults, identities, constraints and indexes, and rejects unsupported schema objects
that would otherwise be omitted. It creates and removes its own scratch SQLite catalog.

[The onboarding plan](../plans/POSTGRESQL-ONBOARDING.md) describes first-run selection, restart
and upgrade handling. This change adds no setup controls or runtime database driver.

Logging: the developer tool prints SQL to stdout and reports generation failures to stderr.
It takes no connection credentials and uses no installation data. There are no new server logs.

Verified with Go 1.27.0 from current main `f2c67db`. PostgreSQL 16.10, 18.0 and 19.0 each
accepted all 44 tables in one transaction, enforced sampled session defaults and constraints,
and left the schema empty after an injected initialization failure. The generator is
deterministic across fresh SQLite catalogs. Go vet, web production build (including app
TypeScript), E2E TypeScript, and the embedded server build passed. The focused SQLite
onboarding journey passed on pinned Chromium after an initial fixture startup timeout.

The first full Go run failed API documentation tests on a missing shared cache entry and
two transcoding tests with system FFmpeg 7. The API documentation package passed with an
isolated Go cache, and both playback tests passed on Jellyfin FFmpeg 8.1.3. The full suite
was not rerun. The optional runtime store, live onboarding, actual Arc schema upgrades,
and backup recovery on PostgreSQL remain untested. All tests used scratch stores.

The PostgreSQL 19 check used the official `postgres:19.0-bookworm` image with networking
disabled. The first attempt hit a Docker storage limit; the schema and rollback checks
passed with a temporary in-memory data directory. The throwaway container was removed.
