# Docker image on Debian trixie (FFmpeg 7.1), so Dolby Vision survives repackaging

The owner's second test (31 October) ran with #318 on the server and the new stats panel on the
Chromecast. The panel showed:
- Arc repackaged the file (its TrueHD needs converting);
- the TV reports Dolby Vision profiles 4/5/8/9;
- yet the video reached the player as plain HEVC (`hvc1.2.4.H150.90`), decoded by
  `c2.amlogic.hevc.decoder` as HDR10;
- the audio change from the same PR was working (TrueHD 7.1 → Dolby Digital Plus 5.1).

## Why

The image's FFmpeg was Debian bookworm's 5.1. FFmpeg's HLS packager copies the video into its own
MP4 writer, and before FFmpeg 6.1 that copy left out the stream's Dolby Vision configuration
(FFmpeg ticket #10490, "HLS packaging does not preserve Dolby Vision metadata"). So
`-strict unofficial` (#318) had nothing to write. FFmpeg 6.1 moved that information into the
copied stream parameters, which is why the same command kept Dolby Vision in the 6.1 test.

## What changed

- **The runtime image is `debian:trixie-slim`** (Debian 13, current stable), whose FFmpeg is 7.1.
  The same packages are installed (FFmpeg, certificates, curl, tzdata, Intel VA drivers on amd64).
  The build stages are unchanged; the server is a static Go binary.
- **A warning when Dolby Vision is lost.** When copied video from a Dolby Vision source comes out
  with no Dolby Vision box in `init.mp4`, the log says so once per run, naming FFmpeg 6.1+. That
  covers installs outside Docker with an older FFmpeg.

## Checks

- Unit test for the warning: once per run, and nothing for kept or non-Dolby Vision video. Go
  build and vet pass.
- **Not built here:** this environment has no Docker daemon and can't reach Debian's mirrors. The
  image needs the manual "Run workflow" with full verification (container installation checks), or
  a build after merge, and then the owner's TV test.
