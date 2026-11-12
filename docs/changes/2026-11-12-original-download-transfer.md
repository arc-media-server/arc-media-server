# Original download transfer (#62, D2)

Add ready manifest and authenticated original-asset GET/HEAD routes independently of playback.
Full/single/open-ended/suffix ranges, exact lengths, safe fixed filenames and private/no-store
headers. Invalid/multiple ranges give416 with bytes */size; originals emit no strong validator,
and If-Range returns full200. Clients replace partial bytes on200 and verify the final SHA-256.

Catalog/source/access guards and bounded admission (one per owner/session, four per server) remain
active through transfer. A joined request-owned monitor revalidates every250ms between available
operations and interrupts blocked socket writes on revocation/shutdown; after headers failures
abort HTTP rather than append JSON. Original files are never changed/copied wholesale. Mutable
external media and blocked filesystem operations retain the documented limits.

Range/manifest regressions failed before the endpoints existed, then pass over real HTTP1.1/2.
Sparse-file checks cover grant/session/release/source revocation while a body is unread, truncated
error semantics and released catalog leases; independent admission/HEAD behavior is checked.
Actual full vet/Go/race outcomes are in the PR. No original-download capability until D3's complete
snapshot/asset handoff, no converted or progress-sync capability, no frontend/native/deployment claim.

Transfer outcomes are rate limited per record/minute with bounded256-key retention. Expected
interruptions are debug-only; no chunk logs, paths, tokens, hashes, bodies, URLs or SQL arguments.
