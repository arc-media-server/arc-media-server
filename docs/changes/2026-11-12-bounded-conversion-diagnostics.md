# Bounded conversion diagnostics

CQR09: optimized-copy conversion reuses playback's existing 16 KiB head/tail stderr capture and first-cause/final-outcome formatter. No general process runner or new dependency. Source paths and URLs are redacted before persisting the failure or reaching the existing warning; cancellation, progress and attempt ownership remain.

A failing portable subprocess emits a cause, 100 KiB of noise and a final error. Its regression failed before the fix (only the final line survived) and passes afterward, including the saved failed-job diagnostic. Existing buffer-bound/privacy tests continue to cover the shared implementation.

Go vet, full Go tests with serial packages, and focused optimizer/transcode race checks pass using prepared Go 1.27.0 and Jellyfin FFmpeg 8.1.3. The first post-fix fixture incorrectly looked for failed jobs through the completed-only lookup; corrected to inspect the failed-job listing and retained the failed attempt. Temporary fixtures only; no production or device claim.

Log volume stays one existing conversion warning per failed job; memory retained per child is at most 16 KiB. No new progress-loop logging or secret/URL/path output.
