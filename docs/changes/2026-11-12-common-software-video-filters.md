# Shared software video filter sequence

CQR08 extracts the existing playback resize-before-tone-map sequence for optimized copies to reuse. Scaling now precedes expensive software HDR mapping in both paths. Playback retains fast/standard selection, captions and hardware upload placement; optimized copies retain their standard tone-map policy and final pixel format. Encoder, stream, bitrate, source/attempt identity and cancellation decisions remain with each manager. No generic conversion runner or dependency.

Prepared Jellyfin FFmpeg 8.1.3 generated HDR10 and HLG inputs are verified to carry actual HDR color tags, then converted by the real optimizer; outputs are 1280x720 SDR/bt709. Existing picture-caption conversions verify resize/standard/fast tone map and burn disabled/enabled; ultrawide outputs retain fitted dimensions. Go vet, full Go tests with serial packages and focused transcode/optimizer race checks pass.

The first new HDR generator produced unknown color primaries/transfer despite command-line flags. Corrected the generated fixture to the existing explicit HEVC VUI recipe, asserting its tags rather than accepting an invalid input. Diagnostic experimentation was removed before commit; bounded-error reuse is the independent CQR09 PR. Earlier unchanged HDR-caption tests failed intermittently in concurrent full runs from other branches; final media/full/race checks pass, but this extraction is not claimed to explain or cure that intermittent failure.

No measured performance/color-quality claim, GPU/device/HDR display acceptance or owner media/settings access. Existing conversion failure logging remains responsible; no extra frame/progress-loop logs, raw URLs, paths or secrets.
