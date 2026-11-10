# #350: distinguish unknown endpoints from unsupported HTTP methods

The path-only /api/ fallback masked net/http's 405 handling, so POST to GET-only item browsing
and other known paths returned 404. Real handler regressions reproduced this in both API
namespaces. The fallback now asks the same mux which standard methods match the path, without
executing any handler, authentication or database work. Return 405 with a sorted Allow header
when a method matches; otherwise retain the JSON 404. GET implies HEAD through ServeMux.

Normal routes and SPA navigation are unchanged. At most nine read-only route lookups occur on
the fallback path, with shallow request copies rather than copied credentials/headers. No
per-request logs or background work added; ordinary 404/405 responses use existing request
handling, not internal-error logging. No secrets, bodies, raw URLs or SQL arguments logged.
Tests cover GET-only paths, a multi-method resource, unknown paths, HEAD, SPA fallback and both
API namespaces. No production systems/data or browser changes.
