# Web: opening Arc afresh starts at "Who's watching?"

**What changed:**
- Opening Arc afresh in a browser now starts at "Who's watching?", even if the last visit left a
  profile in use. A reload, or another Arc tab opened while one is still open, carries on as
  before.
- A new **Sign in automatically** switch (Settings → Your profile → Display) makes this browser
  skip the picker and open as the current profile. It's kept in the browser, not on the server. A
  profile with a PIN can't use it: the PIN is asked for every time.

**How "afresh" is decided:** each open Arc tab marks itself as alive every 30 seconds in the
browser's storage. A page load finding no mark from the last three minutes is a fresh start.
Without browser storage (some private windows) every start is fresh, which is the safe way round.

**Decisions:**
- The picker shows even when there's only one profile; Sign in automatically covers people who
  don't want it.
- The setting is per device, as it's about who uses that device; the other apps get the same
  switch in their own Settings.
- On a fresh start Arc leaves the previous profile (`DELETE /api/auth/profile`), so other Arc tabs
  still open on that browser also return to the picker; that only happens when none of them has
  been active for three minutes.

**Checks:** `npx tsc -b`, the production build, i18n tests, a new browser journey
(`web/e2e/fresh-start.spec.ts`), plus `collection-page` and `poster-size` (all passed). The full
browser suite was stopped at 54 of 190: two `hardware-detection` journeys failed there (they look at
this machine's graphics hardware, not sign-in; not checked on main), the rest passed. The other
journeys start inside Arc via a shared "already open" mark in `playwright.config.ts`.

**Logging:** none needed; profile sign-ins are logged by the server as before.
