# Library controls in navigation

Library navigation now has a per-profile reorder editor, available from desktop sidebar and
mobile navigation. Drag or use keyboard/touch up/down buttons; Save applies the draft, Cancel
discards it, Reset restores server order. Failed saves retain the draft and can be retried.
Only permitted libraries participate; new libraries remain visible.

The sidebar's administrator More menu shares the library page's extracted LibraryMenu, including
Scan, Edit library, metadata refresh, reanalysis, Empty trash and access management. Confirmation
and busy guards are reused. Compact menus do not add item-polling timers. Ordinary profiles
have no administration menus. Settings no longer duplicates Home customization or library order,
and the secondary On this page links are removed; primary/mobile settings navigation remains.

Validation: production build and translation/app TypeScript checks, e2e TypeScript, embedded
server build; three focused browser journeys passed: staged reorder/failure/retry/access at
desktop and phone sizes, settings groups/navigation, sidebar repair confirmation. Updated old
expectations for intentionally removed subsection links. Screenshot inspection confirmed sidebar
placement and settings navigation. No extra logging: existing API failure handling and shared
work outcomes apply; drafts and navigation do not create server logs.
