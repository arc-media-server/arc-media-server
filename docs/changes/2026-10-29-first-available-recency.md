# Recently Added follows first file availability

Episode metadata can be created long before its file arrives. First attachment to a file now
records a durable arrival date and sequence, and sets the item's existing added_at to that date.
The first episode also establishes its show's first-available date. Metadata refreshes and rescans
never bump this event. Existing linked items, including temporarily missing files, retain legacy
dates during migration0058; true historical import times cannot be reconstructed. Metadata-only
rows get their actual date when a file arrives. No filesystem mtime or mutable last_seen value is
used as an import date.

Home episode groups use those dates. Library/Home shows-only/Recommended Added ordering uses each
show's newest present permitted episode; the show's own added_at remains its first availability
for new-show versus new-season grouping. Added-within filters use the same recency definition.
Tied timestamps use a monotonic arrival sequence before title, in both newest/oldest directions.
The Home Recently Added heading opens explicit sort=added rather than a bare remembered library.

Relinking a known file, merging titles and splitting versions preserve the earlier arrival rather
than importing again. A split conservatively inherits its original title's arrival; exact separate
historical dates for each old version were never recorded. Offline/online returns keep the event.
The unchanged-file fast path adds no writes. New association writes stay in the existing short
transaction; there are no new provider requests or per-item info logs.

Validation: focused Store tests demonstrate metadata-before-file correction, opposite chronology,
tie ordering, offline/metadata stability, relink/merge/split preservation and legacy migration.
Scanner/API regressions, focused race/vet, UI build/20 translation checks and two browser journeys
(heading navigation and saved sort direction) all passed. The synthetic migration test initially left its new index behind; its downgrade
fixture was corrected and the complete Store tests rerun successfully. Temporary databases,
generated media/local providers only; no owner catalogue or Plex parity/performance claims.
