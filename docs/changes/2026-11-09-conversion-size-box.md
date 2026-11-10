# #350: fit converted and optimized video inside both quality dimensions

Generated 3840x1600 media reproduced 2592x1080 output in both live conversion and optimized
1080p copies on the Docker-pinned Jellyfin FFmpeg 8.1.3-2 package (8.1.3-Jellyfin binary).
Use one shared FFmpeg scale expression to fit inside the quality's 16:9 width/height box,
retain source aspect ratio, use even output dimensions, and avoid upscaling. Apply the cap
even when source height alone is already below the limit; width may still exceed it.

Real-media regressions failed against the original live/optimizer code, then passed with
1920x800 output. The optimizer's old-code proof uses a Go overlay of the unchanged HEAD file.
Additional FFmpeg cases cover small media, wide media below the height cap, and portrait input.
No hardware encoder, TV decoder or owner-server acceptance claimed. Tone-map ordering and
bitmap subtitle scaling remain separate review items; this change keeps their existing order.

No API fields or UI changes. Existing conversion/optimization outcome diagnostics suffice;
no extra per-frame or progress logs, sensitive values or unbounded diagnostics added.
