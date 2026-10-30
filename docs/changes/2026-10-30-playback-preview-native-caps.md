# Decision-only playback preview and native capabilities

Added a non-probing preview through the same planning path as playback start; it never allocates
streams, conversions or playback state. Unknown saved metadata is explicitly needs_analysis.
Native caps add bit-depth/channel/profile/HDR/passthrough constraints and stable reason codes,
while omitted fields preserve browser defaults. Unsupported explicit HDR/fallback capabilities
are refused rather than claiming playable output. No-video starts now return 422 instead of
reaching a nil video stream. See ../PLAYBACK-PREVIEW-API.md for the app contract and device limits.
