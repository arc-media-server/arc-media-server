# Dashboard names apps as they signed in

The Dashboard showed the Android TV app as "okhttp/5.5.1": devices were named from the
User-Agent, which for the app is its HTTP library's.

## What changed

A playing session now uses the session's device name when it has one: the name an app gave
itself at sign-in, as Settings > Devices shows it (e.g. "Living Room TV"). Browsers have no
device name and are still named from the User-Agent (e.g. "Edge on Windows"). The name comes from
the session the request is already authenticated with, so it costs no extra database read.

## Checks

A unit test plays as an app with the "okhttp/5.5.1" User-Agent and a device name, and as Edge in
a browser, and checks both names.
