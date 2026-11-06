# Conversion pacing reliability — backend handoff

No API endpoints, fields or player clock rules change. Existing `/playing` throttled and
converted_ms fields now clear the throttled flag when the conversion process finishes.
Clients continue normal HLS segment requests and conversion pings; no new client messages.

The old pacing timer could let fast FFmpeg conversions finish several segments before the next
check. It also delayed noticing the next player segment request. This was observable as the
existing throttle regression failing on unchanged main. Segment overshoot can keep a playlist
stationary longer than intended between resumptions; real-device impact has not been proved here.

Each conversion now watches its own generated output directory before FFmpeg starts. Atomic
playlist replacements and valid segment requests wake the existing pacing worker immediately.
It reads current playlist state, rather than retaining an event history. The timer remains a
fallback, and suspension still uses the existing platform process control. No source-media watch,
new service, new dependency, API setting or per-segment info logging is introduced.

Watchers close when the pacing worker exits; failed process starts close the prepared watcher.
Missing notification support and unavailable process suspension log at most one warning each per
manager, without raw paths/arguments. In those cases pacing falls back to periodic checks or the
existing full-speed behavior. Notification scheduling and in-flight work still mean this is not a
hard byte/segment storage quota. The existing stop/seek/admission/idle-cleanup rules remain.

The real generated-media regression keeps its original overshoot/progress/stop assertions. Its
timer is set to one hour so it can pass only through actual output notifications/player wakeups.
Five consecutive runs and transcode race checks passed. Client/device acceptance remains separate.
