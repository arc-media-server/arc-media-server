# #350: de-duplicate and bound urgent preview hints

Repeated requests for the same unavailable preview appended duplicates indefinitely while
previews were off or FFmpeg unavailable. The regression reproduced 1600 entries for one file,
and 1000 distinct requests without a bound. Keep at most 256 distinct urgent file ids, move
requested files to the front as documented, and discard the oldest hint when full. Reject
non-positive ids. The normal library pass still discovers every eligible file; no durable job
or generated asset is discarded. Preview generation, formats and FFmpeg arguments are unchanged.

Concurrent duplicate requests and distinct-file overload tests cover the bounds and priority.
Only temporary cache folders and synthetic file ids are used. No GPU/physical playback claim.
The list mutation is under the existing mutex and does at most 256 comparisons/moves; wake
notifications remain coalesced. No new per-request logs, warnings, goroutines or settings.
Existing preview-job outcome/failure diagnostics remain; no secrets or raw URLs are logged.
