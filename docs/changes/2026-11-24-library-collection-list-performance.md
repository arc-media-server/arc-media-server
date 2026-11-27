# Batch library collection counts and previews

[#524](https://github.com/arc-media-server/arc-media-server/issues/524)

Arc groups visible library collection counts and reads ordered previews in
batches. The previous list counted and loaded members separately for each
collection. The batched results retain profile access checks, minimum sizes,
duplicate suppression, artwork fallbacks and collection grid modes. Smart
collections still evaluate their individual filters.

On the same local fixture of 10,000 movies, 500 series, 20,000 episodes and 600
automatic movie collections, median library collection list time fell from
3.36 s to 198 ms on SQLite and from 4.58 s to 112 ms on PostgreSQL 19. Each
median used five requests without browser rendering or network transfer.

This changes no schema, API contract or logging.

Go vet and the SQLite suite, focused PostgreSQL 19 race coverage, app and
e2e TypeScript, the production web and embedded builds, and the pinned
Chromium collection journey passed. No owner-server timings were taken.
