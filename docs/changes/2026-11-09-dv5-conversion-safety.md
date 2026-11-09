# #350: refuse unverified profile-5 colour conversion

Reverified the report's plausible DV5 colour-safety gap: a native-DV5 player's size/codec/bitrate
conversion could bypass HDR refusal, and optimized copies always used a generic transform.
Policy regressions confirmed the unsafe admission, not actual green/purple output on a real disc.

Refuse profile-5 video re-encoding for live playback (422, preview can_start false) and optimized
copies (an actionable failed-job outcome). Original compatible-device direct play/remux remains
available. Existing DV5 optimized copies made without a verified DV-aware transform are no longer
selected or streamed; artifacts remain removable and source media is untouched. This deliberately
uses the report's conservative refusal option rather than enabling an unproven colour mapper.

Regressions first showed a promised live encode and optimizer admission; fixed cases refuse
before encoder allocation and retain native original playback. Existing-copy selection is covered.
Prepared Docker-pinned Jellyfin FFmpeg package 8.1.3-2 (8.1.3-Jellyfin) is selected in tests; the
safety guard intentionally prevents it encoding DV5. No real DV5 source/display validation or
colour-fidelity claim. Even a build offering tonemapx remains refused until its DV5 pipeline has
specific verified evidence. Known-compatible DV7/8 base playback is a separate PR.

Existing playback validation and optimization outcome diagnostics suffice. Expected live refusal
adds no server error, no per-frame logs, keys/tokens/bodies/raw URLs or SQL arguments.
