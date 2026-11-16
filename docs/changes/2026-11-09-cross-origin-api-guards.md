# #350: guard cookie routes and setup against cross-origin requests

Confirmed with failing real-router regressions on 2d8d4ea: a cross-origin admin cookie could
change settings, unversioned setup reached normal setup handling, Fetch Metadata-only cross-site
requests reached mutations, and all-address home/proxy CIDRs were accepted.

Guard both API route families. Cross-origin unversioned requests return 403 before handlers;
versioned native/TV clients retain Bearer CORS, stripped cookies and disabled home bypass. Honour
browser Fetch Metadata even when Origin is absent. Same-origin checks require matching scheme
and authority; forwarded host metadata is used only from configured trusted proxies. Reject /0
CIDRs for home and trusted-proxy lists. Same-origin cookies and header-free native/script requests
remain supported. No response-field removals or new settings; regenerate API docs after changes.

Regression coverage includes same-site hostile origins, forged forwarded hosts, both setup paths,
missing Origin, direct/default-port scheme matching and a legitimate trusted reverse proxy.
Full vet/test, touched API/auth race and generated contract checks are recorded in the PR/evidence.
No web code or media changes. Logging reuses existing error/request boundaries, with no Origin,
forwarded host, token, key, body or raw URL logging and no new per-request success records.
