# Arc's colour is now deep teal

The owner compared Arc's violet with indigo, blue, teal and magenta (mockups of Home, Settings and
the logo), then three deeper teals, and chose the deepest. Violet felt close to Jellyfin; blue would
move closer still (Jellyfin's highlight is cyan-blue). No media server owns a deep teal, and a cool
colour frames warm poster art rather than competing with it.

- **Colours** (`web/src/index.css`):
  - dark theme: accent `#2bb3a3`, accent-strong `#0b7d72`;
  - light theme: accent `#0b7d72`, accent-strong `#086158`, so text and focus on light backgrounds
    keep their contrast.
- **Logo:** the arc's gradient is `#7fdccd` → `#0b7d72`, in `web/public/icon.svg` and the sidebar
  logo. The PNG icons (180/192/512) are re-rendered from the SVG.
- **Other places:**
  - the API docs page uses Scalar's neutral theme with Arc's teal accent, instead of its purple theme;
  - the README badges;
  - the year-in-review card's gradient ends in teal.
- **Contrast fixes found on the way:**
  - The Activity and News count badges put white text on the light accent. They now use
    accent-strong.
  - Accent buttons hovered to the light accent. They now stay on accent-strong, slightly lighter.
- **Not changed:** profile avatar colours (violet, blue, teal and so on) are each profile's own
  choice, not the brand.

The Android TV app uses the same colours (`clients/android-tv`, Theme.kt). Teal sits near "success
green"; Arc has no green success styling today, so nothing clashes, but new success states should
use a tick and neutral text or a clearly yellower green.
