# The featured banner switch moves to Edit Home

The owner asked for "Show home hero" to live with the rest of Home's layout.

- **Edit Home has a "Show featured banner" switch** at the top, above "Show Up Next separately".
  It's saved with the rows when you press Done, so a failed save keeps the dialog open with the
  error and Done tries again. The admin's Edit Home for another profile (Profiles, Home) has it too;
  Default Home doesn't, because the banner is a profile preference.
- **Settings, Your profile, Home** now has one row, "Rows and featured banner", with an
  "Edit Home…" button, instead of the "Show home hero" switch. "Hero" was jargon; the banner is
  what Plex and most apps call it.
- Library order stays in Settings, Your profile for now: it orders the sidebar, not Home.

Checks: the home hero browser journey now drives the switch in Edit Home (failed save, retry,
hidden after restart, turned back on).
