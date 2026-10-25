# Playback options before Play (#186)

Movies, the next-episode action on a series page and each available episode offer
Playback options. Choose a version, audio track and subtitles, then Play or Resume
inside the dialog. Ordinary Play/Resume keeps its existing automatic behavior.

Opening or cancelling the dialog starts no stream/conversion and changes no profile
preferences or playback progress. Unread source files may be inspected on demand,
as they are when starting playback. Loading and failures have explicit states and
local Retry; Play waits for usable options and profile preferences.

The profile-authenticated `POST /api/items/{id}/playback-options` endpoint uses the
same file/version resolution, source audio DTOs and preferred/default audio logic
as playback. Its input is browser caps plus optional edition/file ID. Its output
contains source file ID, versions, audio/default index and subtitle choices; there
is no stream URL or conversion session. Existing library/age/label access applies.
Source format labels do not promise that the browser outputs those formats.

Automatic audio uses the profile language when available, otherwise the source
default. Automatic subtitles share the player's preference/mode/forced-track
selection helper. Switching versions resets track choices because indices and
subtitle IDs belong to that file. The dialog and player label the selected edition.

Explicit choices travel with that queue item and source file. They apply to the
first playback request and survive initial retry; subsequent seeks/conversion
fallbacks retain the player plan's audio/file and picture-subtitle choice. Picture
subtitles request burn-in on the first start; external picture tracks, which the
browser path cannot burn, are shown disabled with a reason. Actual conversion
availability/failures still come from playback, not a speculative preview promise.

Explicit subtitle Off suppresses automatic selection for that playback. If a chosen
text track disappears before playback, the player explains that it is using profile
defaults. Missing files and invalid audio choices retain server validation rather
than silently switching files. Switching versions inside the player returns to its
existing per-file defaults; preplay IDs are not replayed onto the new version.

Arc's approved language-learning policy is preserved. After successful admission,
explicit audio changes remember the selected language. Explicit full subtitles
remember their language; forced subtitles do not overwrite it. Off clears a learned
subtitle language only in the existing language-driven mode. Cancelling or a failed
start does not learn choices. Automatic selections do not rewrite preferences.

Optimized eligibility is unchanged: matching source/audio/revision/quality may use
a completed copy; mismatched audio, required subtitle burn-in or incompatible limits
fall back to the normal playback path. Real generated multi-track browser coverage
checks French original/remux playback versus matching English optimized playback.

Plex reference: [item details](https://support.plex.tv/articles/202462186-viewing-item-details/)
and [audio/subtitle language settings](https://support.plex.tv/articles/204985278-account-audio-subtitle-language-settings/).
Arc retains its previously approved preference learning rather than silently replacing
it with a new per-item preference model. Casting/native-device behavior remains a
separate acceptance contract (#192/#135); this change targets the browser player.

Actual checks, injected-failure boundaries and remaining device limitations are in
READINESS.md and GitHub #186. No owner data or production services are used by tests.
