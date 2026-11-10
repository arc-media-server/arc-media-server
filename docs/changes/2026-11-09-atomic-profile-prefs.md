# #350: preserve independent concurrent preference patches

The deterministic real-handler regression held one request's body after it read preferences,
let a second request save another field, then released the first. Its stale whole-document
save erased the second change. Decode the bounded request body first, then read/merge/validate
and save the profile's preferences in one short writer transaction. The transform performs only
bounded local validation/JSON work; no body reads, provider calls or other store calls occur
inside it. Response defaults are resolved after commit. Existing patch/reset/nested-style
replacement semantics and full-replacement store helper remain intact.

Tests cover the reported lost update, independent saving while another body is blocked,
validation rollback, explicit false/empty resets, untouched fields, callback-error writer
release and missing-profile rejection. No mutex per profile or new scheduling/background work.
Only temporary stores and synthetic profiles were used. No web changes or owner data.

No new logs: successful saves and ordinary validation need none; database errors use the
existing failure boundary once. Preserve the existing unreadable-stored-preferences warning
without recording raw JSON. No credentials, request bodies, raw URLs or SQL arguments logged.
A transaction serializes these small accepted mutations; this is not a sustained-load or
production performance claim.
