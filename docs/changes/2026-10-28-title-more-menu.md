# Title page More menu for everyone (28 October)

Movie and show pages now show Play (and Play from the beginning when resuming), Playback options,
Trailer, Watched, Watchlist and More. Play next, Add to queue and Add to Playlist moved into More,
which every profile now gets, as Plex's ⋯ menu; admins see a divider and the existing Refresh
metadata, Analyze, Edit, Add to Collection, Fix Match, Optimize and Split Apart below them.
Queue notices and errors still appear beside the button. Episode rows keep their own queue and
playlist buttons. UI only.

Also: Settings → Scheduled tasks' "Tasks can run at any time." sentence is now its own element.
Next to the server clock it made the server-clock journey fail about 1 run in 5 (the clock's
refetch after changing hours raced the assertion); 10/10 repeats pass after.

Checks: tsc app/e2e, production build with the i18n audit, Playwright maintenance, journeys,
recovery, ux, preplay, episode-details and clarity specs (all pass after the fixes above,
including a new journey for a non-admin profile's More menu).
