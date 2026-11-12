# Consistent Library tab label (#80)

Owner decision, 12 November: use Library as the web library's main titles-tab label, matching the
TV terminology. Movie, show and Other videos libraries use the shared translated label. The
library's actual name, media types, routes, filters, sorting and Recommendations remain.

Remove the three unused English tab keys; no backend/API/schema/dependency change. Existing
filter/Home journeys and production/i18n/unit/app checks validate the ordinary browsing flow;
actual verification and the inherited e2e TypeScript limitation are recorded in the PR.
Logging: a translated label has no failure/stall boundary or new log volume; existing loading/
request diagnostics remain responsible. Temporary fixtures only; no owner-server/device claim.
