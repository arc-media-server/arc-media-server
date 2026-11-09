# #350: disconnects must not start a CPU fallback

The real playback-handler regression cancelled a request at first conversion admission and
reproduced two FFmpeg starts: failed hardware readiness then an unwanted CPU retry. It also
logged a hardware failure and returned a server error for an ordinary disconnect. A second
regression showed an already-cancelled request still starting FFmpeg.

Check cancellation before admission, skip CPU fallback for cancelled/deadline contexts, and
stop a session if cancellation occurs while readiness completes. Do not write/log an internal
failure to a departed client. Ordinary active-client hardware failures retain their CPU retry;
no conversion arguments, output formats, API fields or media files change.

Tests use generated H.264/AAC media and the actual selected Jellyfin FFmpeg 8.1.3-Jellyfin at
runtime. The hardware-capability probe is deliberately simulated as previously successful;
this tests cancellation/admission, not a physical GPU. The pinned runtime's first process is
cancelled and its session removed; no CPU retry or hardware-failure warning remains. No owner
media/keys/server, browser or device validation claimed.

Existing successful admission/end logs remain bounded per session. Expected request cancellation
adds no warning/error or extra FFmpeg work; genuine hardware failure diagnostics are unchanged.
No per-segment logs or secret/body/raw-URL/SQL-argument logging is added.
