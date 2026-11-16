# Remaining Web and Server audit review

Owner follow-up: #106/#109 were subsequently accepted for implementation. Their current
contract is in [EPISODE-DETAILS-SPECIALS.md](EPISODE-DETAILS-SPECIALS.md); the original
review/deferral and plan below remain historical. READINESS and the issues record actual checks.

Baseline 49063be, 25 October 2026. This pass covers nine Astra findings from #162;
native/offline apps (F25) and physical HDR/Atmos output acceptance (F26) are excluded.
The current pass implements bounded controls and sender fixes, and records concrete
plans for larger optional projects. It does not equate a review decision with feature delivery.

| Finding | Issues | Decision in this pass |
| --- | --- | --- |
| F21 | #189 | Implement profile library ordering, retain all available libraries; no pin/hide model |
| F22 | #190 | Implement profile completion percentage, existing 90% default; no credit-marker/autoplay change |
| F24 | #106/#109 | Initially planned; subsequently implemented in the owner-authorized #106/#109 follow-up; see EPISODE-DETAILS-SPECIALS.md |
| F27 | #192 | Correct sender file/audio/position ownership, failure cleanup and subtitle-limit feedback; keep open for receiver validation/full subtitle transport |
| F28 | #193 | Defer pacing/budgets until measured resource pressure; preserve existing controls |
| F29 | #138 | Review and consolidate alpha acceptance checklist; keep release gate open |
| F34 | #197 | Owner subsequently accepted Automatic browser quality; implemented via bounded replanning, no managed relay/discovery |
| F35 | #198 | Defer audio processing until representative content/device examples define its benefit |
| F36 | #199 | Implement session-local text-subtitle timing; no automatic analysis or source edits |

## Implemented behavior

### Library order (#189)

Your profile settings offer keyboard-accessible Move up/down and Reset library order.
The order is saved on the server for that profile, shared by desktop/sidebar and phone
navigation. It changes neither Home rows nor permissions. The server's accessible library
list remains authoritative; unknown/inaccessible saved IDs are ignored and new libraries
append in server order. No library is hidden because it is unpinned. Save errors roll back
and expose Retry. Other personal controls share the mutation's busy state. JSON preference
library_order accepts up to 500 distinct positive IDs; [] resets it. Existing defaults remain.
This addresses the task of keeping frequent libraries close without a separate pin system.

### Completion percentage (#190)

Your profile Playback settings offer 75/90/95/100%; 90% remains the default. API clients
can use integer watched_percent 50–100, or 0 for the default. Persisted watch state and
active history/finished notifications use the same boundary. Changes affect future progress
reports, not past plays; marking unwatched preserves historical play counts, as before.
Unknown duration does not complete; the existing API file-duration fallback still applies.
The one-minute resume rule, manual Watched, credits/intro markers, skip behavior and autoplay
remain separate. Choosing 100% does not stop the title before a post-credit scene.
Existing internal ReportProgress callers keep the 90% wrapper; profile-aware HTTP progress
uses the validated preference. No schema migration or historical rewrite is needed.

### Text-subtitle timing (#199)

The player's Subtitles menu offers 0.5s earlier/later and Reset within ±10 seconds for
text captions. Positive means show later. Timing shifts rendered HTML cues and the native
PiP text-track clock; source subtitles/cached cue data are untouched. Off removes timing
controls. Offsets belong to this queue item's playback session and exact file/track: seek,
retry and minimize retain them, another track/file starts at zero, returning to that track
within the session restores its offset, and closing/next queue item resets the session.
Burned image subtitles do not offer this control. No preference write, voice analysis or
automatic synchronization is introduced. Real PiP windows/Safari remain physical checks;
the Chromium fixture tests native TextTrack/VTTCue wiring through simulated PiP events.

### Casting correctness and boundary (#192)

Chromecast requests freeze the actual browser source file/audio/burned-image selection
and keep them across Retry and converted seeks. The old browser conversion is released
before receiver admission. Failed/stale starts release only their owned conversion; Stop/
disconnect release the receiver slot before browser handback. Handback retains exact audio,
including a same-language alternative rather than just the language preference. Request
currentTime compensates for a remux's earlier keyframe. A failed handoff offers Retry.

Chromecast and the app's AirPlay button explain the unsupported browser-rendered text
subtitle path before opening a picker, leaving playback/subtitles intact. They do not silently
turn captions Off. A burned image track is forwarded for server conversion; physical burn-in
output is not newly proven. AirPlay's stream reload retains exact selected audio. Native
Safari/OS-initiated routing and actual receivers remain unverified. Existing conservative
1080p H.264/VP9 capabilities and close-player-stops-casting policy remain; no device-specific
4K/HDR/Atmos capability claim. SDK/receiver and AirPlay API stand-ins verify sender/control
wiring, not real Google/Apple output, permissions, disconnect timing or physical compatibility.

## Larger project plans and acceptance

### Episode credits and actor counts (#106)

The raw TMDB season/aggregate endpoints are requested, but current Episode and Credit
models discard episode guest/crew fields and actor appearance counts. This is not implemented
just because a provider payload contains them. Keep it a separate project:

1. Define provenance: credits follow the show's Details From source, with exact stored
   cross-provider episode mapping. Numbering, titles/descriptions and identity remain under
   the existing canonical policy; do not change them merely to show credits. Missing provider
   credits remain unknown rather than guessed from the local files or another provider.
2. Parse guest cast/director/writer data and supplied aggregate appearance counts. Extend
   existing item-person relations/API with bounded ordering, provenance and optional counts;
   unknown counts are omitted, not presented as zero. No per-episode network request on browse.
3. Add an episode details/credits surface independent of Play and retain existing poster/
   background/Cast layout. Apply profile access and spoiler protection to guest appearances.
4. Verify provider switches, refresh, locks, absent data, duplicate roles, counts, API access,
   keyboard/phone behavior and unchanged episode IDs/history/custom orders with local fixtures.

### Inline specials (#109)

The current metadata Episode model has no stored airs-before hint and the resolved season/
Next Up traversal does not interleave specials. A visual rearrangement alone would make
browsing and autoplay disagree. The plan is:

1. Persist bounded provider placement hints with explicit absence; map by stable episode ID.
2. Offer the option under existing per-show display-order scope/inheritance, with it off by
   default. Preserve source numbering and S00 episode codes; never rename/re-key media.
3. Resolve one display traversal used by seasons, Play/Next Up/autoplay/queue and progress.
   Explicit custom/manual placements win. Unknown or unmapped hints remain in Specials;
   do not duplicate or silently hide episodes.
4. Test before-season/before-episode hints, multiple specials, missing targets, unmatched
   specials, alternate orders, personal/server defaults and preserved watch/history/locks.

These are still requested Later features, not judged unnecessary or counted as completed.

### Conversion pacing/storage (#193)

Current concurrency/temp-folder settings, conversion idle/restart cleanup, optimized-copy
size/delete, artwork pruning and preview deletion remain. No owner CPU/I/O/free-space pressure
was measured. Revisit with a representative concurrent playback/optimization workload and
storage measurements on disposable deployment data. Reuse existing Insights/optimized
reporting; distinguish live conversion, explicit optimization and optional maintenance.
Before adding throttling or eviction define admission, low-space behavior, recovery and which
regenerable assets may be removed. Never delete source media or treat uploaded artwork as
an evictable cache. Acceptance needs measured responsiveness/storage bounds and interruption/
full-storage recovery, not an arbitrary default buffer or simulated-card performance claim.

### Remote quality/discovery (#197)

The owner subsequently accepted adaptive browser playback. Automatic is now the default without
a saved choice, with persistent manual/Original override. Actual HLS delivery and buffer pressure
feed conservative downward/upward hysteresis, grace/cooldowns and bounded single-rendition
replanning. Exact source/audio/burn, absolute position, captions/timing, playback speed and pause
intent survive; serialized admission and current server ceilings remain authoritative. Replans
can briefly rebuffer. Contract: [ADAPTIVE-QUALITY.md](ADAPTIVE-QUALITY.md).

The previous recommendation to defer pending a blocked remote task was superseded by the owner's
explicit choice to have this from the outset. No managed discovery/relay was accepted. Generated
media/local provider tests include real constrained Chromium delivery; upgrade policy tests do
not establish measured remote-network improvement. Actual remote routes, Safari native HLS,
physical receivers and owner-server performance remain unverified. Actual checks: READINESS.

### Dialogue boost/normalization (#198)

No representative owner content/device problem was supplied. Keep audio processing optional
and deferred. First agree a target listening task and whether processing requires conversion;
source TrueHD/Atmos labels do not imply processed output retains that format. Scope one
opt-in mode with Off default, explain output effects and analysis/storage cost, then validate
clipping/loudness, latency/sync, seeking, direct/remux/transcode behavior and opt-out recovery.
Perceived dialogue improvement requires listening/device acceptance beyond numerical fixtures.

## Public-readiness review (#138)

Use [ALPHA-CHECKLIST.md](ALPHA-CHECKLIST.md). Current private image publication and fixture
acceptance do not close real-library/device/recovery/trial or attribution/provider requirements.
No public release is requested or performed. Native clients are separate projects, not a new
web/server readiness gate introduced by this review.

## Evidence boundary and references

Current official Plex navigation, completion, auto-sync and audio-enhancement pages returned
403 during this pass. Its [playback overview](https://support.plex.tv/articles/200430303-streaming-overview/)
was readable and describes app-reported capabilities, remux/conversion and subtitle burn-in.
It does not establish exact current controls, internals or parity for these Arc changes.
Differences chosen here (profile percentage, all-visible order and manual session timing) are
recorded on #80 and the individual issues, with the linked commit and actual validation.

Only generated clips/subtitles/images, temporary stores, pinned Chromium and local providers/
SDK fixtures are used. No owner media/settings/keys, Unraid, installations or persistent server.
READINESS and the GitHub issues/#162 separate actual pass/fail results from historical runs
and still-open feature/device/release requirements.
