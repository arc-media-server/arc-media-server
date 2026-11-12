# Docker build: include the client integration guide

The image's web stage copied only `web/`, but since the client integration guide page (#419)
bundles `docs/CLIENT-INTEGRATION.md` through a `?raw` import, `npm run build` failed inside
Docker ("Could not resolve '../../../docs/CLIENT-INTEGRATION.md?raw'"). Local and CI builds
passed because the whole repository is there. The web stage now copies that one file to
`/src/docs/`, so the relative import resolves. Only that file is copied, so other docs edits
don't invalidate the web layer cache.

Checked by building the web app in a tree holding only `web/` and that file (Docker Hub was
rate-limiting the sandbox, so the image itself is checked by the build-only image workflow).
No logging change: this is a build step.
