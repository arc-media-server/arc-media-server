# Conversion pacing reacts to output and player progress

An existing real-FFmpeg regression intermittently exceeded its overshoot allowance on unchanged
main. Pacing previously depended on timer polling. Session-directory notifications and bounded
player wakeups now trigger current-playlist checks immediately, preserving the existing policy
while reducing polling overshoot and resume delay. Existing fsnotify is reused; resources are
closed on process completion/start failure, and finished processes no longer remain "throttled".
No public API shape changes. See ../CONVERSION-THROTTLING.md for client handoff and limitations.

The unchanged behavioral assertions pass five consecutive real-media runs with a one-hour timer
(to prove event-driven operation), plus full transcode race checks/vet. No source media or owner
settings were accessed; no timing assertions were widened or retries configured. Fallback warnings
are once per manager, with no new info logging in segment loops.

Affected browser checks initially passed four of five. The adaptive pause test sampled playbackRate
before the existing loadedmetadata handler restored it; its immediate assertion saw 1 instead of
1.25. Three independent rechecks passed. The test now polls for the same required 1.25 value before
resuming, matching that asynchronous media lifecycle; no product UI code or expected value changed.
The original failure/trace is retained alongside the final run, not relabelled as a pass.

Final checks: full Go suite (32 tested packages), vet, transcode race, embedded build and e2e TypeScript pass. All five affected browser journeys pass together (2.1 minutes, no retries); the original one-of-five failure and three independent rechecks remain recorded.
