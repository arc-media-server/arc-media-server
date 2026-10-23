# Source video and audio formats

Arc shows Dolby Vision, HDR10+, HDR10 (from PQ transfer) and HLG when supported by the
file's structured FFprobe metadata. Dolby Vision and HDR10+ can both appear for a hybrid
source. HDR10/PQ is the generic label when no dynamic extension has been detected; this is
not a certification of HDR10 mastering or a whole-file proof that dynamic metadata is absent.
No format is guessed from a filename, release name, track title, bit depth or channel count.

Audio uses the codec and, where reported, its profile: Dolby TrueHD / TrueHD Atmos,
Dolby Digital+ / Dolby Digital+ Atmos, DTS / DTS-HD MA / DTS-HD HRA / DTS:X, plus the other
existing codecs. A 7.1 track without an Atmos profile remains ordinary 7.1. Older FFmpeg
versions or stored records may not identify object audio. Codec, profile, channel count and
newly probed channel-layout data are retained per track.

Movie headers describe one source file and its default audio track (or first audio track).
An edition with multiple versions says how many versions it has instead of combining formats;
the file details below describe each version separately. Each series episode has a File details
button that loads its own files/tracks on demand, without a series-wide format claim. Player
source-track/version menus use the same codec vocabulary. Playback method, compatibility,
conversion and tone-mapping decisions are unchanged. Source Dolby Vision/Atmos labels do not
mean a browser, TV or receiver is currently outputting Dolby Vision/Atmos.

## Bounded metadata reads and existing libraries

Migration 0047 adds independent DV/HDR10+ evidence, a declared DV profile where available,
channel layout, optional HDR probe status and an internal format-probe version. It does not
clear probed_at or force every file through a full probe again. Normal backup-before-upgrade
and compatible-backup rollback rules apply.

New/changed files keep their usual basic stream/chapter probe. Structured stream side-data is
read first. Candidate videos (HEVC/H.265, AV1, or PQ/HLG transfer) also receive a compact frame
sample because actual encoded dynamic metadata may be absent from stream-only output. Limits:
64 packets per video stream, at most four candidate streams, one shared four-second wall
budget per file, one decoder thread, 5 MB input probing budget, 2 seconds analysed duration,
2 MiB stdout and 64 KiB stderr per subprocess. FFmpeg input probing limits are not a total file I/O-byte quota.
This does not decode whole films. Multi-stream sampling shares the time budget.

Status `sampled` means frames were examined, `stream` means positive declared evidence,
`unavailable` means the optional sample failed or exceeded a bound. Empty status can mean no
candidate stream. Missing evidence never means the whole bitstream was proven free of dynamic
metadata. Metadata can occur later than the sampled prefix; profile/fallback and hybrid
coverage are not exhaustive. The frame RPU profile is not treated as the consumer DV profile.

Existing successful candidate files enrich gradually: at most **32 per library scan**, using
existing scanner worker concurrency. Compact stream evidence and frame sampling share the
same four-second budget, so declared DV profiles and newly reported audio profiles are captured
without replacing basic metadata. Only files seen during that scan are eligible; unavailable
roots cannot consume the front of every batch. The Activity phase is "Updating format labels"
and these read attempts contribute to file-details-read counts. Basic file metadata, chapters,
matching and watch history are retained. A saved attempt, including unavailable, is skipped
on later unchanged scans. File size/time revisions and concurrent full probes are checked
before saving; a new normal probe wins over stale enrichment.

Large libraries will need multiple scans for the backfill. With scheduled/startup scans off,
manual scans still advance it. Already saved audio profiles need no backfill to display the
richer audio names. Eligible backfilled files refresh structured audio profiles/layouts too; other existing
files retain their saved metadata until a future normal probe. Layouts are not invented from
channel counts. A failed optional attempt is not automatically retried
on every scan; a changed/new file is probed again. There is no new tuning setting or mandatory
library re-add. Real network-drive throughput and CPU/I/O costs still need deployment evidence.

## Verification boundary

See READINESS.md for actual checks. Synthetic profile/UI regressions and encoded DV/HDR10+
metadata samples validate detection and presentation, not HDR appearance, all Dolby Vision
profiles, Atmos pass-through, device compatibility or production deployment. #135 stays open.
