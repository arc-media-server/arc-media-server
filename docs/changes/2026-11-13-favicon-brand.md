# Use the current Arc logo for browser tabs

The HTML favicon still embedded the old purple arc despite the public icon assets already
using the current monochrome TV logo. Point the favicon at `/icon.svg`, which matches
`docs/brand/arc-icon-square.svg`. The manifest and Apple touch icons retain their existing
brand assets. No runtime or logging changes.

Validation: checked the SVG matches the brand source and the production build includes the
favicon reference and asset. No server or physical-device validation is needed for this
static HTML reference change.
