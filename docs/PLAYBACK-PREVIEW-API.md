# Playback decision preview and native capabilities (app handoff)

`POST /api/v1/items/{id}/playback-preview` accepts the same request as `/playback`: caps, optional
file_id/edition, audio_stream, start_ms, max_kbps and burn_subtitle. It shares the actual playback
planning path, including version/audio choice, remote quality limits, subtitle burning and usable
optimized copies. No stream/conversion session, file probe, watch progress, preferences, now-playing
entry or change event is created. This differs from existing playback-options, which may analyse
a file to discover its tracks. Preview does not inspect the source file or test the decoder.

Preview returns the usual playback plan plus `decision_only:true` and `can_start`.
URL is empty, HLS false and session_id absent. Existing plan fields remain intact for v1 apps.
Unanalysed files return `method:"unknown"`, `needs_analysis:true`, `can_start:false`; offer analysis,
then preview again. A fully analysed file without video is unavailable. Conversion unavailable or
an explicitly unsupported fallback returns can_start:false with reason codes; no FFmpeg is started.
can_start is a decision from saved metadata and current server capabilities, not a guarantee of
media integrity, available conversion capacity or real-device output.

New optional caps (send only what the app actually verified):

- `max_bit_depth` (1–16, omitted/0 preserves existing behavior). Explicit 10+ permits Hi10 H.264
  only when H.264 itself is supported; omitted retains the browser's existing Hi10 conversion.
- `max_audio_channels` (1–32, omitted/0 unlimited).
- `video_profiles`: codec -> accepted ffprobe profile names, case-insensitive. Omit a codec key
  to leave it unrestricted. Unknown source profile does not satisfy an explicit whitelist.
- `audio_passthrough`: codec identifiers usable in an original direct file. Remuxed audio still
  uses the existing safe AAC conversion/copy rules; this does not promise TrueHD over HLS.
- `hdr_formats`: hdr10, hlg, hdr10_plus, dolby_vision. **Omitted/null means legacy unspecified;
  [] explicitly means SDR only.** HDR10+ accepts an HDR10-capable client via its base layer.
- `dolby_vision_profiles`: accepted numeric profiles 1–10; when supplied, a DV profile must match.

These are source-decoder/display declarations, not labels proving the actual output. Unsupported
Dolby Vision is conservatively refused unless a compatible optimized copy exists; arbitrary DV
conversion is not promised. Other explicitly unsupported HDR needs enabled and detected tone
mapping. Conversion must fit advertised fMP4/H.264/AAC fallback capabilities; an explicit H.264
profile whitelist must allow High. Optimized copies likewise must fit the new limits.

`reason_codes` is additive to existing English reasons: video_codec, no_video, video_bit_depth,
video_height, video_profile, container, audio_codec, audio_channels, audio_disabled, audio_selection,
hdr_output, subtitle_burn, quality_limit, optimized_copy, conversion_unavailable,
conversion_output_unsupported, hdr_conversion_unavailable, needs_analysis, player_failed (a retry after the player
couldn't play the method it was given; set `failed_method` in caps on the retry). Treat unknown future
codes as generic reasons; localize in the app. Playback start uses the same decisions; unsafe
explicit fallback requests return 422. Do not use preview to allocate a playback URL: call playback
when the user actually starts it.

Logging: previews reuse request failures/debug tracing, with no playback-start info log or mutation
hint. Actual starts retain existing decision logging; bounded validation errors are normal 4xx.
Unit/HTTP fixtures cover legacy/native decisions, no probe/state mutation, dry-run/start agreement,
invalid capabilities and missing-analysis behavior. Browser checks cover existing playback paths.
Actual HDR/DV/Atmos output, native clients and owner-server hardware remain unverified.
