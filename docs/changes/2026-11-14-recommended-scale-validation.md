# Validate Recommended with larger synthetic libraries

Add an opt-in measurement test for the complete authenticated Recommended handler, including
JSON encoding. Each temporary SQLite store contains 10,000 movies and 100,000 episodes:
1,000 shows ×100 episodes or 200 shows ×500 episodes, with orderings, descriptions, shared
cast/directors, multiple genres and approximately 68,000 watch-state records. ANALYZE runs
after seeding. No media files are opened. Normal test runs skip this expensive fixture.

Run explicitly with:

```sh
ARC_RUN_RECOMMENDED_SCALE=1 go test ./internal/api -run '^TestRecommendedScaleMeasurements$' -count=1 -v
```

Baseline main `95fb123` versus PR product commit `12ecf00`, same UTC-day recommendations,
one first request plus five repeated requests per library:

| Shape / library | Baseline repeated median | PR repeated median |
| --- | ---: | ---: |
| 1,000 ×100 / Series | 2,303 ms | 1,228 ms |
| 1,000 ×100 / Movies | 1,107 ms | 1,059 ms |
| 200 ×500 / Series | 2,307 ms | 1,847 ms |
| 200 ×500 / Movies | 1,024 ms | 4,251 ms |

Both runs passed all 24 requests and both shape subtests. Corresponding row counts and payload
sizes matched. Timings became highly variable during the PR's second shape: Series repeated
samples ranged 1,260–12,332 ms and Movies 1,289–24,938 ms. These results do not establish a
reliable long-show or movie speedup; the slower movie result is retained. An earlier PR run
passed with Series medians 1,359/1,340 ms and Movies 778/772 ms, but crossed a different daily
recommendation selection and is supplementary evidence only. No outliers were discarded.

The broad Series case shows about a 47% lower median. Baseline Recently Watched took roughly
700–760 ms, close to the owner's log; PR stage measurements reduce it to tens of milliseconds.
Continue Watching and history/person/genre preparation still account for substantial work.
The smaller initial fixture understated latency at this scale.

These are local in-process measurements with debug logging enabled, not network/Kodi timings
or owner-server acceptance. First request means a fresh store, not dropped OS caches. The two
libraries share their shape's store, with Series measured first. No product code, dependencies,
schema or API changed in this follow-up. No additional runtime logging is needed: the test
uses existing stage diagnostics and emits only synthetic measurement summaries.

Validation: explicit baseline and PR measurements passed; default opt-out skips as intended;
`go vet ./internal/api` and `git diff --check` passed. Earlier full product/race/browser checks
remain in the preceding change note. No owner data, settings, media, keys or Unraid accessed.
