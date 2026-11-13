# README accuracy and setup tidy-up

The README now distinguishes the current web experience from native clients in development
and backend-only download APIs. It removes universal playback and import-speed promises,
corrects shared administrator-managed episode ordering and missing-file visibility, and
clarifies source-format labels versus device output.

Docker instructions now explain container media paths, optional provider credentials,
registry/channel guidance, startup permissions, Docker Desktop networking and current
settings locations. Provider access and licensing link to the maintained distribution guide;
the licence section links directly to LICENSE rather than paraphrasing obligations.
Repeated status/FFmpeg information and internal ownership detail were condensed. The
expandable feature catalogue was removed to keep the README focused on the overview and
getting started. Watch Together and download API guides remain linked for integrations.

Checked against current routes, settings navigation, container entrypoint, Compose example,
provider/distribution guidance and roadmap at 72ec607. Documentation links, image paths,
Markdown fences and shell-example syntax were checked; no server, browser or hardware tests
were run for this documentation-only change. Runtime behaviour and logging are unchanged.
