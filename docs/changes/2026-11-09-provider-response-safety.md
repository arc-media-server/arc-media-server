# #350: keep saved episode data when provider responses are incomplete

Real fixture regressions reproduced HTTP-200 empty, quarter-size and empty-source-with-other-orders
responses. Raw episode counts misleadingly stayed at 96: relinking created file-only replacements
for 95/72 saved identities. Validate the source catalog before writing any fetched metadata/orders.
Reject a missing saved source, zero episodes for an established series, or fewer than half the prior
provider source count. Keep previous orders and report a retryable job failure. Initial file-only
observations are not treated as a prior provider catalog for the half-count rule, so a first match
can legitimately identify only some files. An empty first match still cannot replace known files.

Provider cleanup retains orderings with locked entries or edited season fields; explicit owner
Delete still works. Orphan cleanup retains field locks, labels and both ends of special placements,
in addition to existing file/history/queue/list/collection references. A healthy source with an
omitted edited alternate order retains the owner's placement and emits one summary warning.

Legitimate large provider corrections are conservatively blocked too; this is preservation, not an
assertion the provider is wrong. Review such changes before accepting them; no automatic force path
or new user control is added here. The warning/job error includes safe source/count context, never
URLs or keys. Existing refresh failure boundaries log once; omitted edited orders produce one count
summary per refresh, not a warning per episode/order. No new routine success logs.

Tests first reproduced destructive fallback identities, edited alternate-order deletion and orphan
edit/label/special deletion. Local provider fixtures and temporary SQLite stores only. No API/web,
media writes, production credentials or owner systems. This does not replace the separate #350
canonical episode-id adoption fix or prove real-provider outage/device acceptance.
