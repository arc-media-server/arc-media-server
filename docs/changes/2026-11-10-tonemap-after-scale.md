# #350: resize video before HDR tone mapping

The command regression reproduced HDR-to-SDR filters running before the requested resize. Filter at the target size first, then tone-map, composite SDR picture subtitles, and finally upload for VAAPI if requested. This reduces the pixel dimensions presented to the tone mapper; no elapsed-time or throughput improvement is claimed without a benchmark.

Use the shared aspect-preserving, non-upscaling FitScaleFilter introduced in #368. The picture subtitle canvas is scaled against the already-resized video before overlay, using #370's scale2ref approach; captions therefore stay in position and are not themselves passed through HDR tone mapping. Remux paths do not acquire conversion filters. A zero size limit retains original dimensions.

Integration: #368 and #370 are separate open PRs from the same main. This branch includes their exact shared scale helper and PGS fixture plus the necessary graph composition. Preserve the combined resize -> tone map -> caption-scale/overlay -> hardware upload order when merging; do not restore a second resize after overlay, and retain #368's optimized-copy sizing and #370's pixel regression.

Prepared playback tool tested: jellyfin-ffmpeg package 8.1.3-2, reporting FFmpeg 8.1.3-Jellyfin, the build used for the Docker runtime. Generated HEVC/PQ-tagged black video and generated PGS captions are decoded through both zscale and tonemapx paths, each with/without burn-in. Outputs verify 320x180 SDR/BT.709 and caption pixel rows; the existing 4K/1080p PGS regression also passes. Hardware argument ordering is checked but no physical GPU, display color accuracy, Dolby Vision, or owner-media acceptance is claimed. Synthetic SDR values tagged PQ exercise filter execution, not colorimetric reference accuracy.

The first full-suite run identified old argument-string expectations for the previous graph. Updated those assertions to the intended graph; retain that failed run separately from final validation. No web/API/schema changes or new logging: existing conversion start/outcome boundaries remain responsible, no frame/segment info logs, and no secrets, raw URLs, request bodies or SQL arguments are added.

Final validation: regression failed before the fix; generated-media checks (all four tone-map/burn combinations), Go vet, full Go suite and transcode race checks passed. Transcode race: 37.199s. New media tests name the actual FFmpeg version in their retained output.
