# #350: match picture-subtitle canvas to video before burn-in

The report's plausible bitmap sizing error is now reproduced with a generated PGS stream:
a white caption at row 1000 on a 1920x1080 subtitle canvas over 3840x2160 video ended up at
rows 500..509 after 1080p conversion, rather than rows 1000..1019.

Scale the subtitle plane to its reference video with `scale2ref` before overlaying, then apply
the existing output scale/hardware upload. Normal subtitle placement follows its canvas instead
of assuming the source video and subtitle resolutions match. Keep existing HDR conversion order.

The actual decoded-output pixel regression failed first, then passed on Docker-pinned Jellyfin
FFmpeg package 8.1.3-2 (8.1.3-Jellyfin binary), plus existing CPU/HDR/hardware argument guards.
The test builds tiny PGS palette/object/composition packets itself and muxes temporary generated
video; no discs, owner media or new external tools. Cropped real discs and GPU encoders remain
physical validation limits; do not infer perfect placement for incorrectly authored canvas data.

Existing conversion failure/outcome logging suffices. No per-subtitle/frame logs, sensitive data
or raw URLs added. The filter changes burn-in only, with no API or UI fields changed.
