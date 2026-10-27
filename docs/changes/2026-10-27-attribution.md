# TMDB and TheTVDB attribution (#64, 27 October)

Every profile's Settings → Your profile now ends with About Arc: the server version and a Credits
card. The card gives TMDB's required notice ("This product uses the TMDB API but is not endorsed or
certified by TMDB.") and "Metadata provided by TheTVDB." with a link to thetvdb.com.

Matched movie and show pages end with a quiet line naming where their details came from ("Details
from TMDB", or "Details and episodes from …", with two lines when a show's details and episode
numbering come from different providers). TMDB links go to that title's page on themoviedb.org.
Home videos and unmatched titles show no line.

The official logos are not included yet: the provider sites were blocked from the build
environment. Drop the unmodified files into web/src/assets/attribution/ (tmdb.svg, thetvdb.svg;
see the README there) and they appear automatically next to the names.

Checks: tsc (app and e2e), production build, go vet/test, and 25 journeys in maintenance, clarity
and episode-details specs, including three new attribution journeys. No API, schema or
dependency changes.
