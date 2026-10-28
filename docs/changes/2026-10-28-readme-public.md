# README reads as a public project, and mentions languages

The owner asked for the README to read as if the repository were public, and to say where
translation stands.

- The status badge and note say "alpha" rather than "private alpha".
- Running with Docker no longer asks people to `docker login ghcr.io` first; the same note is gone
  from `docker-compose.yml`.
- The feature list says the app is in English for now, that its text is ready for translation and
  that more languages with a picker are planned (#122), linking docs/TRANSLATION.md.

Limits: pulling the image without signing in only works once the
`ghcr.io/arc-media-server/arc-media-server` package is public (GitHub → the organisation's
Packages → arc-media-server → Package settings → Change visibility). Until then the owner's own
server keeps its existing `docker login`.
