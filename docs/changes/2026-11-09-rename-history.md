# #350: retain identities through ordinary media naming changes

Real scan regressions reproduced a movie gaining a `{tmdb-348}` filename tag and a show folder
gaining `(2002)`: the visible movie/show/episode and media-file ids changed and watched state was
lost. Reconcile a uniquely recognised rename before recording a new file row. Match within the
same library by size/time, compatible parsed title/year/provider hints and (for TV) the same episode
position. Require the old path to be absent, not merely unreadable. Refuse ambiguous or truncated
candidate sets. An indexed, bounded lookup examines at most 33 candidates; no content hashing or
unbounded library walk is added per file. Only catalog paths change; source media is never written.

Retain file ids and their technical data/markers/cache references. Re-key a unique linked top-level
item when its new scanner key is absent, keeping old keys as aliases for remaining versions/folders.
Provider conflicts do not silently re-key existing identities. Explicit new provider hints can retry
an unmatched title through the existing AdoptIDs rules; manual unmatch/metadata locks remain respected.
Identity retention preserves profile history, ratings, playlists, collections and show episode ids
without copying/deleting user state.

Limits: this is conservative rename recognition, not content identity. Changed size/time, ambiguous
fingerprints, unrelated titles or conflicting provider identities are not guessed. Provider episode-id
changes/Fix Match are a separate #350 fix. Tests move only generated scratch fixtures and restore
them before the unchanged-file check. One debug-only reconciliation record carries library/file ids,
no paths, URLs or secrets; existing scan outcome/failure logging remains. No new info-per-file logs.
