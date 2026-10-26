# Automatic browser quality (#197)

Owner-authorized implementation plan, 25 October 2026. Baseline 4917e48.
No managed relay, discovery/account service, owner network configuration or #193 storage policy.

1. Add Automatic to the existing device quality choice; preserve stored manual limits and
   provide explicit Original. Automatic is the default without a saved choice. Adaptation is
   session-local: never save a temporary low bitrate as the viewer's manual preference.
2. Keep original/direct playback until sustained buffer pressure warrants a lower limit.
   Measure actual main HLS fragment delivery and payload rates, including server wait time;
   native/direct playback can step down on sustained stalls but cannot guess an upgrade rate.
3. Use startup/seek grace, sustained pressure, slower upgrades, bitrate headroom, cooldowns,
   a bounded switch window and fresh observations. Ignore pauses, seeks, hidden tabs, receivers,
   pre-roll/extras, near-end playback and terminal errors. Manual choices immediately take over.
4. Reuse the single-rendition planner and serialized conversion admission. Preserve exact file/
   audio/burn selection, text captions/timing, absolute position, volume/rate and local pause
   intent. Release old/stale conversions. Server bandwidth policy remains authoritative on every
   request; expose its independent ceiling to avoid automatic upgrades beyond the current cap.
5. Test the policy deterministically, then actual generated-media browser delivery under network
   constraints, manual override, source/track/seek preservation, pause and error recovery. Keep
   real Safari/receivers/remote-network acceptance separate and record actual results.

This is controlled adaptation through occasional replanning, not a multi-rendition HLS ladder.
Quality changes can briefly rebuffer; they must not restart the title or oscillate every stall.
Do not claim a measured owner-network improvement or seamless switching from cloud fixtures.

## Controls and behavior

Automatic is the browser default when no quality choice was saved. Existing numeric manual
choices remain manual. Original is an explicit saved `0`; Automatic saves `auto`. Older Arc
removed the key for Original, so an absent older preference now uses Automatic. Temporary
adaptations never overwrite the saved mode. The quality menu marks the selected mode and shows
the effective automatic ceiling; choosing a manual quality immediately stops adaptation.

Automatic begins at Original subject to the server policy. After a 12-second startup/seek/
replan grace, buffer below two seconds plus waiting or measured delivery pressure must persist
for six seconds before lowering quality. Fresh HLS main-fragment transfer/payload observations
include server wait time and use an exponentially weighted average. A conservative 65% delivery
allowance accounts for playback speed. Without measurable delivery, persistent waiting permits
one lower step. The ordinary lowest selectable cap is 720 Kbps; a tighter server ceiling wins.
A cap is a conversion target/eligibility limit, not a hard network traffic shaper.

Upgrades require at least two fresh observations, 25 seconds of buffer, 1.8 times delivery
headroom adjusted for playback speed, and 45 seconds of sustained health. Delivery observations
expire after 20 seconds. Lowering has a 20-second cooldown; raising has a 60-second cooldown.
At most four changes occur within two minutes, retained across replans/seeks. Pauses, seeks,
hidden tabs, receiver playback, errors, stopped sessions, extras/pre-roll and the final ten
seconds suppress changes and restart the grace period. Native/direct playback cannot measure
HLS.js delivery, so upgrades wait until measurable HLS playback. The initial decode watchdog
allows 25 seconds in Automatic (15 seconds manually), leaving time for sustained-pressure
adaptation before the existing format-error recovery path.

Each change reuses serialized admission, releases the old owned conversion and rejects/releases
stale responses. It retains file/audio/burn selection, absolute position/keyframe compensation,
text-caption cues/timing, volume, playback speed and pause intent, including a pause while a
replan is awaiting its response. The server returns `server_limit_kbps` independently of the
current request cap and enforces current remote/upload policy on every new request. This is
single-rendition replanning and can briefly rebuffer. It is not seamless multi-rendition ABR,
a bandwidth benchmark or managed remote connectivity.

## Validation boundary

Six dependency-free Node policy checks cover grace, pressure, fresh/stale delivery, conservative
floor, upgrade stability, playback-speed/independent-ceiling behavior, exclusions and the switch
budget. They run directly through Node's test API and are included before Playwright in the
existing browser workflow. A real generated-noise, two-audio-track movie and French sidecar
exercise actual planner/conversion playback under Chromium CDP delivery throttling, plus
manual override. Other focused journeys hold a real admission response to check pause, exact
tracks, caption timing, speed, failure/Retry and seeking, and verify manual/Original/Automatic
persistence on a 390px phone layout. Simulated buffer/time pressure is explicitly limited to
the held-response journey; it is not evidence of real throughput. Upgrade timing/headroom is
verified by policy tests, not a measured owner remote session. Actual counts and failures are
recorded in READINESS and external evidence.

Real remote networks, Safari native HLS, physical receivers, decoder/HDR/Atmos output and
owner-server resource pressure still require their own acceptance. No relay, account/discovery
service, router configuration or production access is added. #193 conversion pacing/storage
and #198 optional audio processing remain separate deferred projects.
