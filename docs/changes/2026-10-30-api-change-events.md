# Bounded API change notifications

Added authenticated SSE invalidation hints, replay/resync, private-profile and library filtering,
admin-only activity/session hints, bounded retention/streams, and session revalidation.
Successful API writes, scan completion and activity updates publish hints without item payloads.
Clients still need fallback refresh; this is not a durable event log or complete database CDC.
See ../EVENTS-API.md for the client contract, limits, logging and remaining validation.
