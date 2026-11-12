# #350: preserve UTF-8 and wire the default event hub

Two reproduced small API findings: device-name truncation at 80 bytes and admin stop-message truncation at 300 bytes split multibyte characters; and Watch Together notification wiring ran before the default event hub existed. Keep the existing byte budgets while truncating only at character boundaries. Short strings remain unchanged. Initialize the chosen/default hub before wiring the callback.

Regression tests exercise real session persistence and the stop handler with strings crossing the byte boundary, as well as default and explicitly supplied event hubs. No API shape or status changes, schema changes or frontend work. No production data or external provider access.

These bounded synchronous operations create no jobs or new logs. Existing session/stop diagnostics remain; personal event hints retain user scoping. No bodies, stop messages, device names, secrets, raw URLs or SQL arguments are newly logged, and successful requests do not add info logs.

Validation: all three regression scenarios failed against unchanged main, then passed. API-doc generation, Go vet, full Go suite and complete API race tests passed (262.217s). No API contract or generated documentation diff; no browser tests needed for these server-only changes.
