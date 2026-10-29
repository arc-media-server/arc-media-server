# Consistent media information and playback methods

Movies and individual episodes now open the same labelled Media information dialog. It reports
stored source information, including overall/video/audio bitrate, aspect ratio, frame rate,
bit depth, Dolby Vision profile, colour fields and HDR inspection status when available. Missing
values are omitted. Versions remain separate; source labels do not establish playback output.
Episode playlist, watched and More controls remain visible without hovering.
Dashboard sessions distinguish Direct Play, Direct Stream (remux) and Transcoding, retaining the
existing throttling indication.

Validation: production build (20 translation catalogs and app TypeScript), e2e TypeScript,
embedded Go server build, two source-format browser journeys (movie at 1440/390px and episode)
and one Dashboard method journey passed with isolated fixtures. The initial Dashboard fixture
used an incorrect response shape; corrected to the actual item/profile response and reran it.
No physical HDR/Dolby Vision output or GPU acceptance is claimed. No additional logs: these are
read-only displays of existing data; errors retain the shared retry UI without poll logging.
Richer mastering metadata via hdrprobe remains a separate dependency decision.
