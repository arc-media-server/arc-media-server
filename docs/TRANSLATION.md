# Translating Arc's web app

Issue [#122](https://github.com/JonzaUK/arc-media-server/issues/122) is being delivered in
separate PRs. Phase 1 provides infrastructure and the shell, sidebar/profile menu, Settings
navigation, Activity/News menus, shared errors/dialog primitives, row navigation and account
screens. Phase 2 converts Home, Watchlist and Playlists, including Home row editing, the playlist
picker/rules and their shared filters. Other page-specific text and feature dialogs follow
in the later area PRs.
English wording, punctuation, capitalization, accessible names and layout must stay identical.

## Adding UI text

Use `useTranslation` in React components so visible text updates when the language changes:

```tsx
const { t } = useTranslation('library');
<Button>{t('actions.scan')}</Button>
<p>{t('added', { title: item.title })}</p>
```

English files live at `web/src/locales/en/<namespace>.json`. `common.json` contains shared
navigation/actions/formatting; `auth.json` contains account and setup text. `home.json`,
`watchlist.json`, `playlists.json` and `filters.json` cover Phase 2. Add `library`, `player`,
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

Languages with more plural categories add their own i18next/CLDR forms. The old `plural()`
format helper is retained for unconverted pages: it selects English noun forms through
an i18next key, but callers still supply English nouns. Replace those callers with complete,
noun-specific keys in each phase covering their area; do not use it for new text. In the final
phase, delete `plural()` and the `format.legacyPlural` catalog entries once all callers are gone.

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
Use `number()` for localized numbers. The English file-size helper preserves its existing
`toFixed` rounding; other locales use `Intl.NumberFormat`. Technical identifiers and codec/HDR
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
Use explicit namespace-qualified keys outside a hook. Keep `Trans` namespaces explicit.

The same tests check English formatting, browser-language selection, on-demand loading and
failed/missing translation fallback. Phase 2 also checks dynamic built-in row/season labels,
filter labels and preservation of literal custom/provider names, saved IDs and genre filters.
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

## What stays English for now

- Areas not yet converted: browsing, title pages and their remaining feature dialogs, Player,
  Settings page contents and management tools. Convert in #122's order,
  about 2–4 pages/components per PR; split Settings into several PRs.
- Server API errors (`writeError`), Activity task titles/stages/details and logs. Existing
  English task-title past-tense conversion and protocol-token matching stay unchanged. A
  later server error-code contract could support client-side localized errors without
  attempting to translate arbitrary server text.
- Provider metadata (titles, descriptions, genres) follows library Metadata language and
  is never passed through app translation keys. User-entered names remain untouched.
- The separate `/docs` API documentation application is out of scope.

After every area is converted, add the agreed rule to CLAUDE.md's Principles or Gotchas:
“All new UI text goes through `t()` and `en.json`.” Here `en.json` means the English namespace
catalogs above. Do not claim the whole app is translated or ready for another language until
that conversion and translation review are complete.

## Deferred metadata-language follow-up

Seasonal rows still filter using English genre display names (`Romance`, `Horror`). A library
whose provider stores localized genre names can miss those rows. In Phase 5 or a separate
metadata follow-up, use TMDB genre IDs or another language-independent key rather than UI or
provider display names. This predates translation extraction and is not fixed here. Tracked
in [Claude's #236 review](https://github.com/JonzaUK/arc-media-server/pull/236#issuecomment-6449183950).
