# API contract coverage: app handoff

This change documents existing response envelopes; it adds no endpoints and requires no UI
changes. `docs/openapi.json` remains the source for generated clients; `docs/API.md` lists routes
and `docs/api-v1-contract.txt` protects response fields against accidental removal/type changes.

Fixed string-key JSON map literals now expose their actual properties rather than an untyped
object. This includes settings, playback/cast helpers, library-health summaries and optimized
versions. Simple helpers returning fixed map literals are included. Branch-dependent keys are
optional unless present in every response branch. Pointers, slices and maps in these envelopes
can be null; clients must handle that alongside their normal type. Dictionary values are typed
where Go provides the type. Contract paths use `{}` for arbitrary dictionary keys and `[]` for
array elements, preserving promises for their children.

Auth status and remote reachability use typed envelopes with the same JSON as before:
unauthenticated auth status includes `profile: null`, `profiles: []` and all eight status keys;
remote check omits `error` on success and includes it on failure. Authentication/authorization
and runtime response meaning are unchanged.

Client work: regenerate models if useful, retain optional/null handling, and do not treat an
invalidation event as a record patch. No forced app migration is needed. Fixed success response
envelopes have a regression guard against silently becoming unstructured again. Dynamic
extensions, including nested ffprobe diagnostics supplied by a callback, remain deliberately
opaque. This is stronger schema coverage, not proof that every runtime/nullability combination
of every historical DTO has been exercised.

Validation: full Go tests, full vet, focused API/apidoc race tests and embedded-server build passed
using prepared tools and isolated fixtures. No product frontend changed, so browser journeys
were not repeated for this branch. Existing v1 baseline lines are retained; generated promises
are additive. No production/server/device acceptance is claimed.

Logging: only documentation analysis and equivalent typed envelopes changed; existing request
failure logging remains. No runtime logging or per-request work was added by schema generation.
