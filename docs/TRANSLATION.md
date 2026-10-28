# Translating Arc's web app

Issue [#122](https://github.com/JonzaUK/arc-media-server/issues/122) prepares the web interface
for translation in separately reviewable areas. English wording, punctuation, capitalization,
accessible names and layout stay unchanged. The final integration branch combines the area PRs
and removes the temporary English-noun compatibility helper. No non-English translation or
language picker is included; first-language review and device validation remain separate.

## Adding UI text

Use `useTranslation` in React components so visible text updates when the language changes:

```tsx
const { t } = useTranslation('library');
<Button>{t('actions.scan')}</Button>
<p>{t('added', { title: item.title })}</p>
```

English files live at `web/src/locales/en/<namespace>.json`. `common.json` contains shared
navigation/actions/formatting; `auth.json` contains account and setup text. `home.json`,
`watchlist.json`, `playlists.json` and `filters.json` cover Phase 2. `library.json`,
`collection.json`, `search.json` and `person.json` cover browsing in Phase 3. Add `player`,
`settings`, etc. as their areas are converted. Namespaces are discovered at build
time, and all English namespaces are bundled. Use readable nested keys (`maintenance.runNow`
in `settings.json`, used as `t('settings:maintenance.runNow')`), never English sentences as keys.
Reuse shared actions with explicit names, e.g. `t('common:actions.cancel')`. Use plain `t`
everywhere; rename local task/item variables instead of aliasing the translation function.

Keep whole sentences together. Use `{{title}}` placeholders with values passed to `t()`;
do not concatenate translated fragments or interpolate the English sentence in JavaScript.
The Home editor has separate `editor.otherTitle` / `otherTitleEndsS` and description keys
for the existing English possessive spelling (Sam's / James'). Other languages may translate
each pair identically when that distinction does not apply; leave the inserted name unchanged.

React handles escaping; i18next's additional HTML escaping is disabled. Never render a
translation or inserted value through `dangerouslySetInnerHTML`.

Use i18next plurals, with numeric `count`, instead of a JavaScript singular/plural condition:

```json
{
  "episodes_one": "{{count}} episode",
  "episodes_other": "{{count}} episodes"
}
```

```tsx
<p>{t('episodes', { count: episodes.length })}</p>
```

Languages with more plural categories add their own i18next/CLDR forms. Use complete,
noun-specific keys with numeric `count`; the legacy `plural()` helper and `legacyPlural` catalog
are removed after all callers convert. Do not assemble translated sentence fragments. Reusable
standalone labels and plural noun phrases (for example a stat tile's unit) can remain separate
where their visual role is independent; a containing sentence must own its word order.

For text containing links or emphasis, keep the sentence in one key and use `Trans`:

```json
{ "help": "Read <guide>the guide</guide> before <strong>continuing</strong>." }
```

```tsx
<Trans ns="library" i18nKey="help"
  components={{ guide: <a href="/guide" />, strong: <strong /> }} />
```

The example is illustrative; use actual app routes. Prefer named components, preserving the
existing DOM and whitespace when extracting rich text.

Dates, language names and numbers belong in `web/src/lib/format.ts`. Its `formatLocale()`
follows the resolved language, with `en-GB` retained for the existing English presentation.
All English browser variants, including en-US, intentionally retain British day-month dates
for now, matching existing behavior. Regional English date/number preferences are a later
owner decision tracked under [#80](https://github.com/JonzaUK/arc-media-server/issues/80).
Use `number()` for localized numbers. The English fixed-decimal helper preserves existing `toFixed` rounding; other locales use
`Intl.NumberFormat`. `localDate`, `localTime` and `localDateTime` retain the pre-existing native
browser English format in the views that used it; translated interfaces follow the active locale.
Explicit British-English timestamp views use `timestamp` with their existing options. Technical identifiers and codec/HDR
brand names remain identifiers, not translated prose. Do not translate route paths, storage
keys, section-anchor identifiers, metadata-matching tokens or API enum values.

## Checks

`cd web && npm run test:i18n` checks every TypeScript/TSX translation reference against all
English namespace JSON files and fails for missing or unused keys. It understands the `t`
property destructured from `useTranslation` (including legacy aliases), explicit `i18n.t('namespace:key')`,
`Trans` with literal `ns`/`i18nKey`, and plural families. Use static keys; dynamic concatenation
is rejected. Compound key segments use camelCase; long lowercase segments are rejected unless
explicitly documented as genuine single words. Counted words require one/other forms; attached
unit abbreviations can remain invariant. Use an explicit lookup of literal translation calls for enum-to-label mappings.
Browsing uses stable internal unit identifiers (`movie`, `show`, `video`, `title`) with complete
noun-specific count keys; never pass translated nouns or metadata through that lookup.
Person credit grouping uses the role data, never the translated heading. Filter menus use full
“Any …” phrases rather than lowercasing translated field labels.
Use explicit namespace-qualified keys outside a hook. Keep `Trans` namespaces explicit.

The same tests check English formatting, browser-language selection, on-demand loading and
failed/missing translation fallback. Phase 2 also checks dynamic built-in row/season labels,
filter labels and preservation of literal custom/provider names, saved IDs and genre filters.
Phase 3 checks noun-specific browsing counts and unchanged internal unit identifiers, plus
desktop/phone browsing, collection actions, Search and Person credits with literal metadata.
Use `rowTitle()` when showing a built-in Home row; never translate custom titles by matching
their English text. Seasonal dates use the shared `month()` helper. They run as part of `npm run build`, requiring no new
test dependency. For each area PR also run production and e2e TypeScript, production build,
`go vet ./...`, `go test ./...`, and its affected browser journeys (full suite for Phase 1).
Compare desktop and phone screenshots with main. Existing Playwright English selectors must
remain unchanged: correct the extracted text if they break.

## Adding a language later

No non-English translation or language picker ships in these groundwork/area phases. When a reviewed first
translation is ready, add `web/src/locales/<language>/<namespace>.json` using the same keys
and placeholders as English. Use a BCP 47 language tag, such as `fr` or `pt-BR`. Rebuild:
Vite discovers languages and creates separate chunks automatically. The browser detector
reads `navigator.languages`; it does not use URL, cookie or local-storage overrides. Only
languages with files in the build can be selected. English appears synchronously on first
paint and remains usable while available translations load. Missing namespace/key files
or failed chunks fall back to English; absent languages issue no locale requests. Changing
or adding language files requires a new build because the UI is embedded in the server.

Weblate comes later, once the repository is public. It will edit these same JSON files and
submit PRs; no translation backend service or provider key is needed by Arc. A per-profile
language picker and right-to-left layout work are deferred until a first translation exists.

## Coverage and review order

The area PRs are review units; the final combined PR is the integration/cleanup review. Review
the areas before merging the combined result. Keep #122 open until the owner/Claude review is
complete. The combined commit retains the area heads as parents so merge ancestry records the
reviewed work; do not squash or separately merge overlapping snapshots without reviewing conflicts.

| Area | Components / behavior | Review |
| --- | --- | --- |
| Shell and authentication | Sidebar/profile menu, shared states/dialogs, Activity/News, sign-in/setup/invites | Merged #235 |
| Home and saved lists | Home, rows, editor, playlists, Watchlist, filters | Merged #236 |
| Browsing | Library, Search, collections, person credits | #238 |
| Titles | Movie/Series, episode cards/details, credits, scores, file/source information, queue/watched actions | #240 |
| Metadata dialogs | Artwork, Fix Match, markers, chips | #241 |
| Details and orders | Edit Details/Season, file/display order, comparison | #242 |
| Title actions | Collections dialogs, bulk actions, optimized versions, preplay | #243 |
| Player | Playback controls/client notices, queue/chapters, casting/AirPlay, subtitle search and track labels | #244 |
| Profile settings | Preferences, subtitle style, password, About and attribution | #245 |
| Server settings | Library/playback options, diagnostics, notifications, scheduling and maintenance | #246 |
| Access settings | Home/remote networking, HTTPS/ports, devices and history imports | #247 |
| Library editor | Add/edit, folders, source order/language, preview | #248 |
| Library management | Scans, removal/repair confirmations, health, marker coverage | #249 |
| Reports | Dashboard, Insights, Year, resource graphs, playback compatibility | #250 |
| Administration | Profiles/friends/invites, PINs/restrictions, backup policy/restore | #251 |

The final cleanup also checks client-generated errors/notices, whole phrases, stable Settings
anchors/group keys and keyboard event tokens. `Home` and `End` event values, matching tokens,
API enums and stored identifiers must never use translated labels. Catalog checks catch a
translated keyboard comparison as well as dynamic/missing/unused keys and plural families.
A static text audit is evidence of reviewed coverage, not proof that every future runtime state
or language is accepted. See the dated area/integration notes for actual checks and limitations.

## What stays English or unchanged

- Server API errors (`writeError`), Activity task titles/stages/details and logs. Existing
  English task-title past-tense conversion and protocol-token matching stay unchanged. A
  later server error-code contract could support client-side localized errors without
  attempting to translate arbitrary server text.
- Provider metadata (titles, descriptions, genres), which follows library Metadata language;
  user-entered names, file paths, device values and source ordering names from providers.
- Technical identifiers, codec/HDR brands, hardware/software names, routes and stored keys.
- The separate `/docs` API documentation application, which is out of scope.

All new UI text goes through `t()` and the English namespace catalogs (`en.json` in the
original #122 guidance). Rich text uses `Trans`. Do not introduce a language picker, RTL work
or a translation backend as part of this extraction. The first reviewed language must check
runtime language changes, document `lang`, missing-key fallback, language-specific plural forms,
wrapping/overflow, accessibility and real-device behavior; the English-only checks do not
establish those outcomes. Weblate remains deferred until the repository is public.
