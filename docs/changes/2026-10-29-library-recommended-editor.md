# Per-library Recommended layout

Administrators can arrange, hide/show and choose card styles for Recommended categories in each
library. The layout is shared by profiles using that library; each profile's watch history and
permissions still determine row content. Home is independent. Dynamic categories (Because you
watched, genres, actors/directors) retain stable family IDs rather than saving changing titles.
New/omitted categories remain visible by default; empty categories appear when content exists.
Save is explicit, Cancel discards drafts, failed saves preserve drafts, and Reset restores default
order/visibility/styles. Arbitrary new custom recommendation queries are outside this change.

The editor extracts Home's pointer/touch drag and keyboard movement into ArrangeRow and shares
its labels/styles. The existing Home editor retains its other controls. Layout is stored in an
indexed, cascading library_recommended table (migration 0059; 0058 reserved by the recency PR).
Administrator GET/PUT /libraries/{id}/recommended/layout validates a bounded list of known,
unique row families and card styles. Recommended reads apply it after existing permission-aware
content generation. Deleting a library removes its layout through the foreign key.

Validation: API package passed; store package passed after changing the table to WITHOUT ROWID
so its primary key is an indexed foreign key; focused API layout tests passed under race;
API/store vet; production translation/app TypeScript, e2e TypeScript and embedded build passed.
Browser journeys passed for Recommended staging/save/reset, phone layout, failed-save recovery
and Home's shared keyboard reorder/filter controls. Phone screenshot inspected and a long label
wrap fixed. All data/providers were temporary fixtures. No owner deployment acceptance.

Logging: one info outcome when an administrator saves, with library and bounded family count.
Read/poll paths add no logs; invalid input is an ordinary validation response; database failures
retain the API error boundary. No request bodies, title lists or credentials are logged.
