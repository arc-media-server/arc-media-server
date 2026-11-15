# First-run setup: clearer providers, keyless note, background import notice

Part of #436 (first-run setup polish). Small wording and feedback changes; the flow is unchanged
(administrator profile → optional providers → first library).

- **Provider wording.** "Both have free API keys" is gone: it promised something Arc can't
  guarantee, especially for TheTVDB, whose access terms are still an open decision (#80). The
  description now says what the keys are for and where to change them later (Settings, under
  Metadata providers). TheTVDB's link reads "About TheTVDB API access" instead of "Get a key".
- **Skipping providers.** With both fields empty, a note says what still works: Arc adds and
  plays the files, using their names and any artwork or .nfo files beside them (the scanner
  reads both), and how to look titles up later (add a key, refresh the library).
- **After adding a library.** The Libraries page shows a notice that the library is added but
  its media is still being found and matched in the background, and that the Activity icon
  beside the notifications bell shows progress and problems. It appears for any new library,
  not only the first.
- **Accessibility.** Password errors are announced (`role="alert"`) and linked to their fields
  (`aria-describedby`, `aria-invalid`); setup errors are announced too.

Not changed, still open on #436: setup saves provider keys without checking them, so a mistyped
key only shows up later as failed matches. Checking keys needs a server-side test call and
is left for a separate change. The Advanced help on file episode order and the home sign-in
wording were read and left as they are.

Validation: `npx tsc -b`, production build, and the journeys, ux, clarity and i18n browser
suites (82 passed). A fixture walkthrough on a throwaway server (temporary data and media
folders, no provider keys): password error, providers step on desktop and phone width, the
first library added through the folder picker and the notice shown. No logging changes (web
text and feedback only).
