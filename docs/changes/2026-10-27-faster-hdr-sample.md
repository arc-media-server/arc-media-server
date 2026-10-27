# Faster HDR sample (27 October 2026, PR #226)

The Dolby Vision/HDR10+ frame sample decodes keyframes only (`-skip_frame nokey`, same 64
packets) and falls back to the full decode if ffprobe rejects the option. Generated DV 8.1 and
HDR10+ clips (made with the dolby_vision/hdr10plus crates; metadata on every frame, keyframes,
the first frame or only between keyframes) gave identical detection at ~85 ms instead of
~415 ms per 1080p file. Tiny real fixtures live in internal/probe/testdata (its README explains
how they were made). Contract: docs/SOURCE-FORMATS.md.
