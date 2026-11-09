# The Docker image uses jellyfin-ffmpeg 8.1 (7 November)

## What changed

- **The image's FFmpeg** is Jellyfin's build, jellyfin-ffmpeg v8.1.3-2 (FFmpeg 8.1.3 with
  media-server patches), in place of Debian trixie's FFmpeg 7.1.
  - It's installed from Jellyfin's release `.deb` for trixie, amd64 or arm64.
  - It's pinned by version and checked by SHA-256.
  - It's linked as `/usr/local/bin/ffmpeg` and `ffprobe`, so Arc finds it as before.
  - The source of that line is mirrored in arc-media-server/arc-ffmpeg.
- **It brings its own video drivers**: libva, Intel's iHD driver and VPL runtime, Mesa and Vulkan,
  newer than Debian's. Debian's `intel-media-va-driver` is no longer installed. `i965-va-driver`
  stays for older Intel graphics, and libva still looks in the system's driver folder.
- **Faster HDR to SDR.** Hardware detection now notices jellyfin-ffmpeg's `tonemapx` filter
  (`transcoder.fast_tone_map`, additive). When it's there, conversions that need tone mapping use
  it instead of the zscale chain. tonemapx is SIMD-optimised and many times faster on the CPU, maps
  with BT.2390, and applies Dolby Vision's own colour metadata where a stream carries it. Without it
  (a plain FFmpeg on a PC install), the zscale chain stays.
- Chromaprint (intro detection) and zimg are part of the package.

## Why

It's the most capable open FFmpeg for a media server, kept current with FFmpeg's releases (8.1).
- It's the quickest route to faster HDR conversion, today on the CPU.
- It's the base for GPU tone mapping on Intel, NVIDIA and AMD later.
- It's the base for Jellyfin's Dolby Vision work (profile 7 FEL tone mapping is on its
  `feat/dovi-fel-tonemappers` branch).

The base image stays Debian trixie-slim. The package supports it, and changing distribution would
gain nothing.

## Limits

- **Not built here.** The image couldn't be built in the agent's environment (Debian's package
  servers aren't reachable there), so the container checks run in GitHub's manual Docker image
  workflow.
- **The tonemapx filter options** follow Jellyfin's own use and the filter's source (BT.2390,
  bt709 output, tv range, yuv420p). Its speed and picture haven't been compared on the owner's
  files.
- **GPU tone mapping** (OpenCL, VAAPI, QSV, CUDA) isn't used yet. It needs a path per vendor, and
  Intel's OpenCL runtime in the image.
- **Dolby Vision profile 5** still isn't converted (`hdr_conversion_unavailable`), although tonemapx
  can apply its metadata. That's a follow-up once checked on a real file.

## Logging

Hardware detection already logs its outcome. The new `fast_tone_map` flag is in the transcoder
status the Dashboard reads. Each conversion's FFmpeg arguments stay at debug level as before.
