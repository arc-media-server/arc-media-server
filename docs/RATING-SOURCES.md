# Rating sources: IMDb pause and future decision

Owner decision, 29 October 2026: pause IMDb ratings while investigating repeated multi-minute
write stalls. This is a containment measure; diagnostics do not prove IMDb caused the incidents,
and disabling it is not a proven fix for the later hanging library-list requests.

## Current behaviour

- The ratings manager neither downloads IMDb datasets nor reads/applies an existing cached file.
  There is no setting or environment override to re-enable that background path.
- The web UI hides IMDb scores, vote counts, sort choices and the settings row advertising weekly
  updates. An old library browsing URL/preference using `sort=imdb` falls back to title order.
- IMDb IDs remain for filename matching, metadata identity and subtitle queries. They are separate
  from IMDb rating ingestion and must not be removed as part of a rating cleanup.
- Metadata-provider audience scores, personal ratings, and optional OMDb Rotten Tomatoes/Metacritic
  scores remain. Missing provider scores remain missing; this adds no cross-provider backfill.
- Existing IMDb rating columns, cached files, store methods, API response fields and backend sort
  support remain for compatibility. Previously saved smart filters can still use the backend sort;
  the web offers no new IMDb sort selection. No destructive migration or cache deletion occurs.

## Deferred: remove the IMDb rating implementation?

Revisit with the owner after evaluating sources. If IMDb ratings will not return, remove the unused
fetch/parser/store code, fixture override and any remaining obsolete docs. Decide separately
whether API deprecation and eventual cache/schema cleanup are worthwhile; preserve IDs and avoid
an expensive startup migration merely to clear old scores. If retaining ingestion, first verify
licensing/attribution, add indexed lookups and bounded write batches, and test responsiveness on a
large synthetic library. Never restore the previous unbounded all-title write transaction.

## Source options investigated (official material checked 29 October 2026)

| Option | Practical fit | Terms and remaining checks | Recommendation |
| --- | --- | --- | --- |
| Metadata-provider score, including TMDB | Already stored and displayed; no new ratings downloader, account or matching layer. Scores/coverage differ from IMDb. | TMDB allows non-commercial API use with attribution; commercial projects need its commercial arrangement. Review logos/notices under existing attribution work. | Use the existing score now; no new cross-provider fetch. |
| OMDb critic scores | Already supported using the owner's key, with per-title requests and rate limiting. | OMDb labels content CC BY-NC 4.0 and has additional legal terms; a paid key alone is not proof of rights for every upstream brand or commercial use. Verify attribution, caching, distribution and intended use before public release. | Retain current optional integration; assess terms before expanding. |
| IMDb scores via OMDb | Technically possible without a full IMDb dataset pass; current Arc parser intentionally reads only critic scores. Adds API quota and missing-value constraints. | OMDb is not affiliated with IMDb. Do not assume a different endpoint grants broader IMDb redistribution/display rights; confirm applicable permissions with providers. | Investigate only; not implemented or approved as a replacement. |
| Licensed IMDb data | A direct route if actual IMDb scores become a product requirement. | IMDb offers licensing including user ratings; pricing and permitted caching/display/distribution depend on an agreement. No quote requested. | Consider later only if the benefit justifies cost and complexity. |
| Personal ratings only | Already supported; no external rating-data dependency. | Personal scores are distinct from provider audience/critic scores. | Sufficient where an external score is absent; never label them IMDb. |

Do not scrape IMDb/critic websites or assume Arc's AGPL licence grants rights to third-party data.
The review above records published terms, not a legal determination for a future business model.
Before adding any source, check movie/series/episode coverage, stable IDs, provenance, attribution,
quotas, cache/update/delete obligations and deployment/distribution rights. Use a local fixture and
bounded database writes; measure user-action latency during refreshes.

## Sources

- [IMDb non-commercial datasets](https://data.imdb.com/non-commercial-datasets/)
- [IMDb: use of data in software](https://help.imdb.com/article/imdb/general-information/can-i-use-imdb-data-in-my-software/G5JTRESSHJBBHTGX)
- [IMDb content licensing](https://help.imdb.com/article/imdb/general-information/content-licensing/GZGA5HDQ8NE97LVR)
- [TMDB API FAQ and attribution](https://developer.themoviedb.org/docs/faq)
- [OMDb API and its content licence notice](https://www.omdbapi.com/)
- [OMDb legal terms](https://www.omdbapi.com/legal.htm)
