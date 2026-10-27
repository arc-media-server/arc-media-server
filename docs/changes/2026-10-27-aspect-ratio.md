# Aspect ratio (#212, 27 October 2026, PR #225)

Every video track stores its display aspect ratio (migration 0055, `media_streams.aspect_ratio`):
FFprobe's display ratio, else the frame size corrected by the pixel shape, so anamorphic DVDs
read 16:9. File details show e.g. "1920×800 · 2.40:1" (4:3 and 16:9 by name).
`store.FormatVersion` is now 2: scans save it for new and changed files, and the Media analysis
task brings every older video (HDR or not) up to date once with the compact stream read,
keeping saved HDR details. Contract: docs/SOURCE-FORMATS.md.
