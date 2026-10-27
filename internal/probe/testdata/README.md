# Probe fixtures

Two-second 128x72 10-bit PQ HEVC clips (a plain grey picture, keyframe every 24 frames) with
real dynamic HDR metadata on the frames **between** keyframes only, the hardest case for a
keyframes-only sample:

- `dv81-other-frames.mkv`: Dolby Vision profile 8.1 RPUs (NAL type 62), generated with the
  `dolby_vision` crate's `GenerateConfig` and appended to each tagged access unit.
- `hdr10plus-other-frames.mkv`: HDR10+ (SMPTE ST 2094-40) prefix SEI, encoded with the
  `hdr10plus` crate.

The base clip was encoded with ffmpeg/libx265 (`hdr10=1`, BT.2020/PQ, master-display and
max-cll), the metadata spliced in the way `dovi_tool inject-rpu` and `hdr10plus_tool inject`
do, then muxed with `ffmpeg -c copy` (HEVC -> MP4 -> MKV). They contain no real content.
