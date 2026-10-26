# Library metadata, file order and display order

Owner-approved simplification, 26 October 2026. Replaces the earlier per-show Details From
choice and the old season-area ordering menu. Investigation: unequal provider catalogues,
single-show refresh scope and the Recently Added episode-sample cutoff.

## Controls

| Location | Control | Behavior |
| --- | --- | --- |
| TV library, Advanced | Metadata provider | TMDB (default) or TheTVDB for show-level title, description, artwork, cast and details |
| TV library, Advanced | File episode order | Default interpretation of filenames; also selects episode titles, descriptions, stills, runtime and supported credits from that numbering provider |
| Edit Show, Metadata & ordering | File episode order | Library default or per-show TMDB aired / TVDB aired, DVD or absolute override |
| Edit Show, Metadata & ordering | Display order | Presentation/playback traversal of identified episodes; Follow file order or another available provider/custom order |

The season area has an Episode ordering… shortcut to this same dialog. Ordinary profiles
can change display order/specials for themselves; file numbering and library metadata remain
administrator-only. The administrator's display choice also sets the existing server default.
Use server default restores profile inheritance. Show-specials inheritance and stable mapped
placement hints remain; custom/manual placements keep their behavior.

File order, display order and specials changes are staged until Save. Cancel leaves them alone.
Compare Orders retains its explicit Use action and recoverable failure/Retry; Use applies the
selected view immediately. Save/Retry failures stay visible in the dialog. There is no separate
per-show metadata-provider control. Episode credits now follow episode/file provider rather
than the library's show provider; unsupported/missing provider credits stay empty.

## Safe refresh scope

Show numbering changes, individual series/episode refresh and cross-provider display mapping
reserve the existing per-library job slot. Busy work returns a conflict before the scoped
mutation; request cancellation, Stop and queued discovery use the existing job identity and
coordination. Provider/mapping errors return an error, rather than only logging a warning and
reporting success. Numbering/view settings saved before a provider failure can remain selected;
Retry must perform the refresh again, including when the requested numbering already equals
the saved value. This is not an atomic rollback of every metadata/provider request.

Refresh synchronizes the selected show's metadata and relinks its saved present files by name.
It never enumerates library folders or calls ffprobe. Missing files stay missing. New/unrelated
files remain for discovery. Stable identities, history/resume, file links, manual field locks
and custom placements retain the existing adoption/rekey rules. Activity reports the selected
show's refresh and supports Stop without duplicate API/scanner refresh tasks.

Library provider/file-order changes refresh known titles using the existing bounded manual
metadata operation; folder changes still request discovery. Naming-only edits need no scan.
Metadata-language behavior and other background schedules retain their existing policy.

## Catalogue gaps and provider availability

Changing display order does not reinterpret filenames. An alternate order can only place
identities mapped to local episodes. A genuinely absent episode in the canonical provider
cannot be created merely by selecting a different view. Use the file order matching the actual
filenames; do not silently merge metadata providers or guess identities from the chosen view.
Existing provider-availability/show-absence fallback remains; selecting a provider cannot
create information or credentials it does not have. Provider failures now surface on scoped
changes. Live-provider completeness, Duncanville's actual catalogue/mappings and the owner's
pre-change settings/files were not accessed or validated here.

The dedicated fixture has 16 files, 11 TMDB catalogue episodes and 16 TVDB episodes. TMDB file
numbering leaves five file-only episodes unplaced in TVDB view; changing only the library's
show provider does not resolve that. TVDB file numbering with TMDB show information places all
16 while retaining file-only episode identity, locked title and resume, with zero discovered/
probed files. A season-count difference in a real catalogue need not mean episodes are absent
from its other seasons/orders; that requires actual provider evidence.

## Upgrade and API compatibility

Migration 0053 adds `libraries.details_provider`: empty means TMDB, `tvdb` means TheTVDB.
Existing libraries start with the historical TMDB show default. The migration does not change
stored metadata/artwork, numbering, identities, files, history or locks, and starts no work.
Previously saved per-show `items.details_provider` values remain in the database for rollback
but no longer affect refresh. Later explicit/scheduled refresh uses the library provider;
choose TheTVDB once in library settings if that is the desired show-information source.

Library POST/PATCH accepts `details_provider`; TVDB is valid only for TV libraries. The old
per-show PATCH field is rejected before applying metadata edits, with guidance to library
settings. `source_ordering`, display-order and specials APIs retain their identifiers. Old
clients repeating a failed source change receive a real retry, not a false successful no-op.
The pre-upgrade backup is checked read-only without migrating the backup itself.

## Recently Added grouping

The grouped TV Home row no longer takes 400 episode records before grouping. The store chooses
the most recent eligible shows first, ranked by their latest eligible episode with stable ties,
then returns complete eligible episode sets for those candidates. The API applies the requested
card limit after existing show/season/episode and temporal grouping. At most the requested number
of show candidates is loaded; there is no arbitrary episode cutoff hiding older show candidates.
This bounds show candidates, not a strict memory limit for a very large selected show.

Selecting at most N recent shows is sufficient for N cards: any later show's newest possible
card is older than at least N cards from those candidates. Existing seven-day grouping, new-show
detection, profile display-order season positions and access restrictions remain. Complete
groups retain accurate episode counts. Six shows with 100 present episodes each now return six
complete 100-episode cards instead of four. Movies and the optional one-card-per-show query keep
their existing behavior. No owner-library performance benchmark is claimed.

## Evidence

Focused regression coverage includes the actual grouped endpoint with 600 saved episodes,
denied-library exclusion, 11/16 provider fixtures, file-only identity/history/lock preservation,
busy/Stop coordination, real provider-error/Retry responses, legacy-field rejection, schema-52
upgrade/backup safety, library provider persistence and browser Save/Cancel/Retry/profile/phone
flows. Actual counts and earlier failures are recorded in READINESS and external evidence at
`/workspace/arc-cloud-evidence/metadata-scan-review/`. Earlier successful feature suites are
historical evidence, not proof of this change. No owner data/settings/media/keys, Unraid, new
tools/dependencies or persistent server.
