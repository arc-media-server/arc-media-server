# #350: bound unauthenticated input and pending device links

Failing regressions showed that oversized setup/login/profile/friend/invite/link/password bodies
reached handlers and that a single client could reserve more than five pending link slots.

Limit credential/setup/link JSON reads to 16 KiB using MaxBytesReader. Oversized bodies return
the existing safe 400 JSON validation response. Device names are at most 256 UTF-8 bytes. Keep
the global 100-slot device-link cap and add a five-slot cap per canonical client IP (trusted proxy
rules still apply). Expired slots are pruned before both counts. Shared-address households can
have five pending links at once; approval/collection or the ten-minute expiry releases capacity.

Set the HTTP server's total request-read timeout to 60 seconds and keep-alive idle timeout to
two minutes, in addition to its ten-second header timeout. No WriteTimeout is added: long-lived
SSE/playback responses remain possible. Slow uploads must complete their request body inside the
read deadline. This is a configuration bound, not a slow-network/load benchmark.

Focused real-handler regressions cover all eight input paths and another client retaining link
capacity. No API fields removed or browser changes. Existing validation/rejection responses diagnose
failures; no new routine logs, request bodies, keys, codes, poll tokens or raw URLs are recorded.
