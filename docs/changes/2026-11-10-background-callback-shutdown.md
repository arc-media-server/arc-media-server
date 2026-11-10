# Own webhook and artwork callback lifetimes

Webhook deliveries previously used a detached Background context, and the artwork
keeper exposed no way to join its loop. They could outlive the store on shutdown.

Notifier.NewWithContext binds asynchronous sends to the process. Close stops
admission, cancels deliveries and joins admitted network/store work. Normal root
cancellation does not generate failure warnings. Keeper.Close cancels and joins
source-listing, prefetch and prune work; a queued trigger cannot start another
listing after cancellation. Main supplies the root and defers both closers before
the store closes, including startup-error returns.

The report's broad worker-lifetime finding was originally based on code reading.
New deterministic lifecycle tests block a delivery transport/source listing,
observe cancellation, prove Close waits until they unwind, and reject later work.
No live providers or owner data are used. Main's other loops still need the
separate process-worker drain; this does not claim API Low 9 complete.

Preserve #356's safe webhook errors and #373's retry fixes during integration.
No new logging volume, raw URLs/keys/bodies/SQL arguments, API/schema, dependency,
frontend or media changes. Part of #350.

Validation: go vet ./..., go test ./... and serial touched-package races passed:
notify 5.475s, artwork 1.036s; cmd/ams has no tests.
