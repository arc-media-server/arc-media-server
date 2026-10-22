# UI review regression coverage

The original four diagnostic checks reproduced the findings in `docs/UI-REVIEW.md`. After the
owner approved implementation, their assertions were replaced with desired-behaviour checks
in `web/e2e/ux.spec.ts`, alongside broader usability regressions. They now run in the ordinary
release suite against the real embedded app and the existing isolated fixture supervisor.

From `web/`:

```
npm run test:e2e
```

Go, FFmpeg and Playwright Chromium are required, as for the ordinary browser journeys. Ports
18199/18200 must be free. The supervisor generates disposable data and local provider fixtures;
never repoint it at owner data. Requests to outside services are blocked in the browser.

Screenshots and traces go to ignored `test-results/`. The custom-row check substitutes a
synthetic 60-match API listing; this does not measure scanning throughput.
The mobile check uses desktop Chromium at a 390px viewport, not an actual mobile device.

The obsolete diagnostic configuration and assertions have been removed; they must not be used
as acceptance evidence after the fixes. The review report preserves the historical observations.
