# Dashboard: stream details, hardware conversion and how far ahead

The owner wanted the Dashboard to show more (31 October): whether a conversion uses the graphics
card, and the kind of detail Plex shows per stream. The TV app's stats panel covers the player's
side; this is the server's side.

## What changed

Each playing session now records how it's sent (`stream` in `GET /playing`, additive):
- **Reasons** it isn't played as it is (as already logged).
- **Video and audio from the file to the player:** e.g. "HEVC 4K Dolby Vision (as it is)" and
  "Dolby TrueHD 7.1 → Dolby Digital Plus 5.1".
- **The encoder** when video is converted: the graphics card by name (NVIDIA NVENC, Intel Quick
  Sync…) shown as a **Hardware** label, or **CPU (software)**.
- **HDR mapped to SDR**, and **subtitles drawn into the video**, when they apply.
- **How far it has converted** (`converted_ms`): "Converted to 1:02:30 (2:00 ahead)". This shows
  whether a spinning player is waiting on the server (barely ahead) or on the network or the
  player (well ahead).

The cards are now one per row, up to very wide screens, so the details have room.

## Checks

- Unit tests for the descriptions: direct play, repackaged with E-AC-3, CPU conversion with tone
  mapping and burned subtitles, a named graphics card.
- The Dashboard browser journey checks each line and the hardware label.
- The v1 contract was regenerated (`stream`, `converted_ms`).
