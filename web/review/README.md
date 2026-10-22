# Diagnostic UI review

These four checks reproduce observations from `docs/UI-REVIEW.md` against the real embedded app
and the existing isolated fixture supervisor. They deliberately assert the observed gaps: a
passing diagnostic means reproduction succeeded, not that the product is correct. They are
excluded from the normal `e2e` directory and release regression suite.

From `web/`, after `npm run build`:

```
node node_modules/typescript/bin/tsc -p review/tsconfig.json
node node_modules/@playwright/test/cli.js test --config playwright.review.config.ts
```

Go, FFmpeg and Playwright Chromium are required, as for the ordinary browser journeys. Ports
18199/18200 must be free. The supervisor generates disposable data and local provider fixtures;
never repoint it at owner data. Requests to outside services are blocked in the browser.

Screenshots, traces and JSON observations go to ignored `test-results/ui-review/`. One custom-row
check substitutes a synthetic 60-match API listing; this does not measure scanning throughput.
The mobile check uses desktop Chromium at a 390px viewport, not an actual mobile device.

When fixing each issue, convert its reproduction into a normal regression asserting the desired
behaviour, and remove/update the corresponding diagnostic expectation. Do not weaken the release
suite to keep an obsolete diagnostic passing. No product code is changed by this review harness.
