# Dolby Vision survives remuxing; surround sound when converting audio

From the owner's first test on a Chromecast with Google TV (31 October). A Dolby Vision profile 8
film with TrueHD audio was direct streamed: remuxed, because the Chromecast can't take TrueHD.
It played as HDR10 with stereo sound.

## What changed

- **Dolby Vision is kept when remuxing.** FFmpeg writes a copied stream's Dolby Vision
  configuration box (dvcC/dvvC) into MP4, including Arc's fMP4 HLS, only when allowed
  "unofficial" boxes. Without it, players see plain HEVC and show the HDR10 base layer. Copied
  video now gets `-strict unofficial`; streams without Dolby Vision are unaffected.
- **Surround sound is kept when converting audio.**
  - Before, any audio that needed converting became AAC stereo.
  - Now, when the player says it decodes Dolby Digital Plus (or Dolby Digital), multichannel audio
    it can't take as it is (TrueHD, DTS, multichannel AAC, FLAC…) becomes E-AC-3 (or AC-3) at
    640 kbit/s, up to 5.1, as Plex does. Atmos in TrueHD can't be kept this way.
  - Dolby Digital (Plus) audio the player decodes is now copied instead of converted to stereo.
  - Streams limited by quality (away from home, or a lower quality chosen) keep AAC stereo, to save
    bandwidth.
- **`audio_output` in playback answers** (`/api/v1`, additive): the codec, channels and whether Arc
  converted it. This is for the players' stats panels and the Dashboard.

## Checks

- A synthetic Dolby Vision profile 8.1 file (an HEVC MP4 with a hand-added dvvC box) went through
  Arc's exact remux command. Without the flag, `init.mp4` had no Dolby Vision box; with it,
  ffprobe reads the configuration back (profile 8).
- An MKV with that video and 7.1 audio came out as HEVC `hvc1` with Dolby Vision plus E-AC-3 5.1,
  with FFmpeg 6.1. The Docker image's FFmpeg (Debian bookworm's 5.1) has the same `-strict` rule.
- Unit tests cover the arguments and the audio decisions (Dolby decoder or not, stereo players,
  copied E-AC-3). The `webserver` and `preplay` browser journeys pass.

## Limits

- Dolby Vision on the TV still needs a real-device check. The box is now present, and the TV's
  decoder then decides.
- Profile 5 files keep the `hvc1` tag when copied. Players that require `dvh1` for profile 5 may
  still show them wrongly (no change from before).
- The `transcode started` log line now includes `audio_codec`.
