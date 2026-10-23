# Settings navigation consolidation

Owner approved, 23 October 2026. Tracked in #166.

1. Keep Home, Watchlist, Playlists and viewing libraries in everyday navigation.
   Replace the desktop Manage group and mobile Manage menu with a clear Settings entry.
   Retain the profile menu's Settings shortcut for every profile.
2. Give Settings and existing management pages the same persistent navigation:
   personal preferences; Server (Dashboard, Insights, Library Health); Libraries
   (Manage libraries, Libraries & metadata); Playback; People & access (Profiles,
   Access & devices); Maintenance (Backups, Maintenance & diagnostics).
   Preserve the Library Health count and all current section deep links.
3. Keep dedicated pages and current URLs, including query parameters for Insights
   and library editing. Do not flatten tools into a long settings form. Desktop uses
   a grouped navigation rail; phones use a grouped section selector.
4. Add View Dashboard in the admin Activity popover, including empty/loading states.
   Following the link closes the popover. Dashboard itself keeps its Activity list.
5. Ordinary profiles retain personal settings only. Management routes must not mount
   admin page queries for them; existing API authorization remains authoritative.
6. Verify the production build and app/e2e TypeScript, then focused browser journeys
   for desktop/phone navigation, active state, existing URLs/history, health count,
   Activity linking and ordinary-profile restrictions. Inspect actual screenshots.
   Use temporary stores/generated clips/local providers only; do not repeat unrelated
   Go/race tests for a navigation-only change. Keep results and handoffs explicit.

Build/release policy changes and the deferred Plex-review fixes remain separate work.

Implemented with shared route layout and navigation metadata, retaining existing page
components and URLs. Production/app/e2e TypeScript and five focused browser journeys pass;
actual evidence and initial corrections are recorded in READINESS and cloud evidence.
