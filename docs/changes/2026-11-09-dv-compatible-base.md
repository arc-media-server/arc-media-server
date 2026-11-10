# #350: compatible Dolby Vision base-layer playback

Reproduced capability decisions refusing profile 7/8 compatible bases as unsupported Dolby Vision.
Store structured `dv_bl_signal_compatibility_id` evidence as an optional stream field (including
explicit zero); carry it through scan and format enrichment. Analysis version 3 brings older
files up to date using the existing Media analysis task, without a metadata/library rescan.
Unknown profile 8 or explicit zero remains unsupported; do not guess from PQ alone.

For a player explicitly reporting HDR capabilities without the relevant DV profile, use only a
known compatible base: P7 compatibility 6 (legacy P7 without the field also has an HDR10 base),
P8 compatibility 1 HDR10, 2 SDR, or 4 HLG. Repackage with Jellyfin FFmpeg 8's `dovi_rpu=strip=1`
so unsupported DV metadata is not handed to an HDR10/HLG/SDR-only decoder. This deliberately
uses remux rather than assuming a particular client's direct player ignores DV. SDR clients
can tone-map the compatible HDR base using the existing conversion path; SDR bases need no
HDR mapping. Native supported DV is unchanged. Profile 5 safety is a separate review item.

Regression decisions failed first, then passed. The existing generated two-second grey DV8.1
RPU fixture was remuxed with DV stripped while retaining HEVC/PQ and converted to H.264/BT.709
using the Docker-pinned Jellyfin package 8.1.3-2 (binary 8.1.3-Jellyfin). Compatibility parsing,
explicit-zero storage/JSON and capability rejection are covered. No real discs, devices,
profile 7 enhancement-layer fidelity, HLG or P8.2 display acceptance is claimed. Older external
FFmpeg builds without dovi_rpu cannot provide this remux path; Docker uses the checked build.

Existing single playback-decision/session outcome logs explain the base-layer choice through
reason code `dolby_vision_base_layer`. No new per-frame/segment logs, sensitive fields or raw
URLs. Stored source labels still describe the source, not the physical display output.
