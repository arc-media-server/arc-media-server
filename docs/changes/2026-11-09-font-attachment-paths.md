# #350: choose font attachment output paths inside Arc

Rechecked the report at main d94283c after #349. The exact pinned amd64 package
jellyfin-ffmpeg8 8.1.3-2 (FFmpeg 8.1.3-Jellyfin), SHA-256
82bd7422530740af738891c54b548eadf9bb572ff875976eddbe231e6f02482b, already refuses unsafe
attachment filenames: the original outside-cache write does not reproduce on this build.
Arc nevertheless used an empty dump filename and delegated output paths to media tags. A new
command-boundary regression failed before the change; real generated absolute/traversal/Windows
separator fixtures verify the no-outside-write boundary with the shipped FFmpeg.

List attachments with companion FFprobe, accept only portable font basenames, bound advertised
font sizes, and supply each attachment index and complete Arc-selected output path explicitly.
Extraction uses unique temporary directories and atomic cache publication; old cache keys are
invalidated, partial/cancelled extraction is not cached, and non-regular/oversized cached files
are ignored. Existing benign font names and embedded ASS output remain unchanged. Media is read
only; all generated fixtures and sentinels live in temporary directories.

Logging: no new per-font/runtime logging and no metadata paths, keys or raw URLs in errors.
Generic extraction errors reach the existing caller; no subtitle/media payload is logged.

Actual full vet/test and touched-package race results are recorded in the PR and evidence under
/workspace/arc-cloud-evidence/review-350. No web or /api/v1 shape changes, installs to system paths,
owner media/data or production/Unraid access. This is independent path containment, not a claim
that the report's FFmpeg 6.1.1 exploit reproduces with Jellyfin 8.
