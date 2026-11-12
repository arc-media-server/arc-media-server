# #350: pre-roll paths must select regular video files

A real ordinary-profile HTTP regression reproduced a configured JSON file and a video-named
symlink to it being served through both API namespaces. Apply the existing folder video-extension
policy to explicit files too, and require regular files with Lstat: no file symlinks, pipes,
devices or directories masquerading as videos. Keep MP4/M4V/MOV/WebM/MKV, case-insensitively.
Folder entries use the same check. Invalid/unavailable selections are skipped; an absent index
continues returning the existing generic 404.

Pre-rolls deliberately remain administrator-selected, server-wide assets outside libraries:
forcing them into a library would break the existing separate-clips workflow. This is a regular-
file/extension guard, not deep codec/content validation or library-scoped access control. A
linked video's real file path can be configured instead; directory aliases remain supported.
The administrator still controls which legitimate clips are offered to every profile.

Tests cover the reproduced non-video/link exposure, folder filtering and a regular configured
outside-library clip remaining accessible. Synthetic fixture bytes/temporary folders only;
no physical playback, owner files or production settings accessed. No new per-request logs:
invalid selections are expected configuration, documented in the client handoff. Existing
request diagnostics remain; no keys, bodies, raw URLs or SQL arguments are recorded. Directory
enumeration remains synchronous as before; candidate regular-file checks add filesystem stats,
not FFmpeg/probing work, background tasks or unbounded retention.
