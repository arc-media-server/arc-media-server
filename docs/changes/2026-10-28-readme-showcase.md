# README showcase layout and screenshot tool (28 October)

The owner asked for a more eye-catching README that talks about what Arc is rather than listing
features. The README now opens with the logo, tagline, badges and a home screenshot, then "What
is Arc?", a grid of four screens (film, show, library, profiles) and "What makes it different".
The full feature list is kept in a collapsed "Everything Arc can do" section; install, build,
API, credits and licence sections are unchanged.

`web/tools/screenshots.mjs` makes the five images in `docs/screenshots`: a throwaway Arc with a
library named after Creative Commons (Blender Studio) and public-domain films and shows, matched
on TMDB/TheTVDB with the caller's keys (`--tmdb`/`--tvdb` or AMS_TMDB_API_KEY/AMS_TVDB_API_KEY),
some profiles and watching, then the home, film, show, library and profile screens in dark mode
at 1600×1000. It never touches the owner's data or media. Rerun it after UI changes.
The current images were taken in a cloud session with the owner's keys set as environment
variables. Spring (2019) has no TMDB poster; it only shows in the cut-off bottom library row.
