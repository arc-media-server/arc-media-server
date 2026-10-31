# Packaged TV apps can call /api/v1 and find Arc on the network

The owner approved an LG webOS and Samsung Tizen app (31 October): one TV web app, packaged as an
`.ipk` and a `.wgt`, in its own repository, talking to Arc only through `/api/v1`. Two small
server gaps stood in its way.

## What changed

- **Cross-origin access to /api/v1** (`internal/api/app_cors.go`). A packaged TV app runs from its
  own local origin (`file://` or `null`), so every call it makes is cross-origin, and Arc sent no
  CORS headers outside cast links. Now any origin may call `/api/v1`, with a preflight answer for
  `Authorization`, `Content-Type`, `Range` and `If-None-Match`. To keep the browser's sign-in safe,
  a cross-origin call is treated as an app:
  - its cookies are dropped, and `Access-Control-Allow-Credentials` is never sent, so only a
    Bearer token (from linking with a code or signing in) can authorise it;
  - "sign in at home without a password" never applies to it, so a web page someone opens on the
    home network can't list profiles or pick one;
  - `POST /api/v1/auth/setup` is refused cross-origin (first-run setup stays in Arc's web app).
  Same-origin calls (Arc's API docs page) and the web app's unversioned `/api/...` are unchanged.
- **Discovery** (`GET /api/v1/discover`, additive). TV web apps can't send or receive UDP
  broadcasts the way Plex's and Jellyfin's native clients find servers, so the TV app will try the
  addresses on its own network on Arc's port. `/discover` answers without signing in, only to the
  home network (404 from outside), with `product: "arc"`, the server's id, name, version and API
  version. Manual address entry remains the fallback.
- **apidoc** now recognises a handler that streams with `io.WriteString` (the WebVTT endpoint), which
  had stopped `go run ./tools/apidoc` with "no success response found" on main.

## Decisions (logged on #80)

- Discovery is an HTTP probe, not a UDP broadcast, because the TV runtimes give third-party web
  apps no UDP sockets. A UDP responder can be added later for the Android app if wanted.
- Allow any origin rather than an allow-list: packaged apps report `null`/`file://`, which an
  allow-list can't distinguish from a sandboxed page anyway; safety comes from ignoring cookies and
  the home bypass instead.

## Media playback

The TV's native player (`<video>` on webOS, AVPlay on Tizen) can't add an `Authorization` header.
No server change was needed: the app asks `POST /api/v1/cast` for a link with the token in the
path, as casting does, and those links already allow any origin.

## Limits

Checked with Go tests only. How each TV runtime treats cross-origin requests from a packaged app
(webOS's file origin, Tizen's `config.xml` `<access>` rules) still needs checking on a real TV.

## Logging

No new logging: preflights and refused setup calls are ordinary validation, and requests still
go through the existing request log (debug level, warnings for failures and slow requests).
