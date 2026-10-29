# Arc for Android TV

The Arc Media Server app for Android TV and Google TV: sign in with a code, pick a profile, browse
Home and your libraries, and play. Native: Kotlin, Jetpack Compose for TV and Media3 (ExoPlayer).
It talks only to Arc's stable API, `/api/v1` (see `docs/API.md` at the top of the repository).

Status: a first working slice (milestone 18, issue #63). What it does and doesn't do yet is in
`docs/changes/2026-10-29-android-tv-app.md`.

## Build

You need JDK 21 and the Android SDK (platform 37, platform-tools). Then, in this folder:

```sh
./gradlew assembleDebug          # app/build/outputs/apk/debug/app-debug.apk
./gradlew testDebugUnitTest      # unit tests (and the screenshot tests, without saving images)
./gradlew recordRoborazziDebug   # renders every screen at 1920x1080 into app/screenshots/
```

Release builds (`assembleRelease`) are signed with the debug key for now: there are no signing
keys in git. Arc's Go server and web app build exactly as before; nothing here is part of them,
and `clients/` is left out of the Docker image.

### Setting up the SDK on a new machine

- **Windows (the owner's PCs):** install Android Studio (`winget install Google.AndroidStudio`),
  open `clients/android-tv` in it and let it install the SDK it asks for; or, without the IDE,
  download the "command line tools only" zip from developer.android.com, unzip it to
  `%LOCALAPPDATA%\Android\Sdk\cmdline-tools\latest`, and run
  `sdkmanager "platform-tools" "platforms;android-37.0" "build-tools;36.0.0"`. Set `ANDROID_HOME`
  to the SDK folder (or put `sdk.dir=...` in `local.properties`), then use `gradlew.bat`.
- **Linux (and Claude's containers):** `tools/setup-sdk.sh [sdk-dir]` installs the command-line
  tools and the same packages; then `export ANDROID_HOME=<sdk-dir>`.

## Install on a TV

1. On the TV: Settings > Device Preferences > About, press OK on **Build** seven times to turn on
   developer options; then in Developer options turn on **USB debugging** / **Network debugging**.
   Note the TV's IP address (Settings > Network).
2. From a computer on the same network:
   ```sh
   adb connect <tv-ip>:5555        # accept the prompt on the TV the first time
   adb install -r app/build/outputs/apk/debug/app-debug.apk
   ```
   Or sideload the APK with an app such as *Send Files to TV* or *Downloader*.
3. Arc appears in the TV's apps row (with its banner). `adb logcat -s Arc` shows what it logs.

## Point it at your server

On first start the app asks for the server's address: what you'd type in a browser at home, e.g.
`192.168.1.20`. Port 8099 is assumed unless you type one; `https://arc.example.com` uses HTTPS on
its usual port (remote access). The app checks `GET /api/v1/health` before going on.

Then it shows a four-character code. In Arc in a browser, open the profile menu, choose **Link a
Device** (or go to `<server>/link`) and enter it. The TV is signed in as the household (or as a
friend, if a friend approves it), and asks who's watching; profiles with a PIN get a number pad.
The sign-in appears in Arc's Settings > Devices, where it can be signed out.

## Layout

```
app/src/main/java/org/arcmediaserver/tv/
  ArcApplication.kt, AppContainer.kt   app-wide objects (session, HTTP clients, image loader, caps)
  data/        ArcApi (the /api/v1 client), Models, ServerAddress, SessionStore (Keystore-encrypted
               token), HomeRows (the profile's Home layout, resolved as the web app does), ArcLog
  playback/    DeviceCapabilities (what this device plays), PlaybackClock (offset_ms, seeks, subtitles)
  ui/          one package per screen: launch, server, signin, profiles, browse (Home, libraries,
               sidebar), details (movies, episodes, a show's seasons), player; components and theme
app/src/main/res/values/strings.xml    every word the app shows
app/src/test/                          unit tests, screenshot tests, and the live end-to-end test
```

## Testing against a real Arc

`LiveServerTest` drives the API client through a whole visit against a running Arc: TV sign-in
code (approved as the admin, as the browser would), a wrong and a right PIN, Home rows, a library,
a show's seasons, a direct-play decision with a ranged read of the file, a converted HLS stream
part-way in with its playlist and first segment, progress, the keep-alive ping and the session's
end. It's skipped unless pointed at a server. Use a scratch Arc, never a real one:

```sh
# at the repository root
cd web && npm install && npm run build && cd ..
go build -o /tmp/ams ./cmd/ams
go run ./tools/testmedia -out /tmp/arc-tv-media
AMS_LISTEN=127.0.0.1:8199 /tmp/ams -data /tmp/arc-tv-data -config none &
# set it up: an admin with a password, the test library, and a profile "Sam" with PIN 2468
curl -c /tmp/j -b /tmp/j -X POST localhost:8199/api/auth/setup -d '{"name":"Admin","password":"test-pass-123"}'
curl -b /tmp/j -X POST localhost:8199/api/libraries -d '{"name":"Movies","kind":"movies","paths":["/tmp/arc-tv-media/Movies"]}'
curl -b /tmp/j -X POST localhost:8199/api/libraries -d '{"name":"TV","kind":"shows","paths":["/tmp/arc-tv-media/TV"]}'
curl -b /tmp/j -X POST localhost:8199/api/profiles -d '{"name":"Sam","color":"teal","pin":"2468"}'
# then, here
ARC_E2E_URL=http://127.0.0.1:8199 ARC_E2E_PASSWORD=test-pass-123 \
  ./gradlew testDebugUnitTest --tests '*LiveServerTest*' --rerun -i | grep E2E:
```

## Screenshots

`./gradlew recordRoborazziDebug` renders the screens on the JVM (Robolectric) at a TV's
1920x1080 with made-up titles and generated artwork, into `app/screenshots/tv-*.png` (not kept in
git). Look at them after changing the UI: an emulator isn't needed.
