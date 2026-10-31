import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import ts from 'typescript';
import { createInstance } from 'i18next';

const root = fileURLToPath(new URL('../src/', import.meta.url));
const i18n = createInstance();
await i18n.init({ lng: 'en', fallbackLng: 'en', resources: { en: Object.fromEntries(['common', 'home', 'playlists', 'filters'].map(ns => [ns, JSON.parse(readFileSync(root + `locales/en/${ns}.json`, 'utf8'))])) }, interpolation: { escapeValue: false } });
function load(file) {
  const compiled = ts.transpileModule(readFileSync(root + file, 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText;
  const exports = {};
  new Function('require', 'exports', compiled)(name => {
    if (name === '@/i18n' || name === '../i18n') return { default: i18n };
    if (name === '@tanstack/react-query' || name === '@/lib/api') return {};
    throw new Error(name);
  }, exports);
  return exports;
}
const home = load('lib/home.ts');
const filters = load('lib/filter-labels.ts');
const format = load('lib/format.ts');
const library = { id: 7, name: 'Continue Watching <&>', kind: 'movies' };
const saved = [
  { id: 'custom:fixture', title: 'Continue Watching', filter: { library_id: 7, genre: 'Horror', sort: 'added' } },
  { id: 'playlist:fixture', title: "Owner's <&> playlist" },
  { id: 'collection:fixture', title: 'Horror for Halloween' },
];
test('English built-in labels, seasonal dates and filter summaries preserve their existing text', () => {
  const rows = home.resolveRows(saved, [library]);
  assert.equal(home.rowTitle(rows.find(row => row.id === 'continue')), 'Continue Watching');
  assert.equal(home.rowTitle(rows.find(row => row.id === 'recent:7')), 'Recently Added in Continue Watching <&>');
  assert.equal(home.currentSeason(new Date(2026, 9, 27)).title, 'Horror for Halloween');
  assert.equal(format.month(2), 'Feb');
  assert.equal(format.month(10), 'Oct');
  assert.equal(filters.sortSummary('imdb'), 'by title');
  assert.equal(Object.hasOwn(filters.sortLabels(), 'imdb'), false);
  assert.equal(filters.sortLabels().critics, 'Rotten Tomatoes');
  assert.equal(i18n.t('filters:preview.movies', { count: 1 }), '1 movie matches');
  assert.equal(i18n.t('filters:preview.movies', { count: 2 }), '2 movies match');
  assert.equal(i18n.t('playlists:titleCount', { count: 1 }), '1 title');
  assert.equal(i18n.t('playlists:titleCount', { count: 2 }), '2 titles');
  assert.equal(i18n.t('home:rows.seasonEpisodes', { season: 'Season 1', count: 1 }), 'Season 1 · 1 new episode');
  assert.equal(i18n.t('home:rows.seasonEpisodes', { season: 'Season 1', count: 2 }), 'Season 1 · 2 new episodes');
});
test('language changes update built-ins without translating metadata, custom names, IDs or saved filters', async () => {
  const rows = home.resolveRows(saved, [library]);
  const before = home.toSaved(rows);
  i18n.addResourceBundle('de', 'home', { rows: { continue: 'fixture-continue', recent: 'fixture-recent {{name}}' }, seasonal: { horror: 'fixture-horror' } });
  i18n.addResourceBundle('de', 'filters', { sort: { title: 'fixture-title' } });
  await i18n.changeLanguage('de');
  try {
    assert.equal(home.rowTitle(rows.find(row => row.id === 'continue')), 'fixture-continue');
    assert.equal(home.rowTitle(rows.find(row => row.id === 'recent:7')), 'fixture-recent Continue Watching <&>');
    assert.equal(home.currentSeason(new Date(2026, 9, 27)).title, 'fixture-horror');
    for (const item of saved) assert.equal(home.rowTitle(rows.find(row => row.id === item.id)), item.title);
    assert.deepEqual(home.toSaved(rows), before);
    const resolved = home.resolveRows(saved, [library]);
    assert.deepEqual(home.toSaved(resolved), before);
    assert.equal(resolved.find(row => row.id === saved[0].id).filter.genre, 'Horror');
    assert.equal(filters.sortLabels().title, 'fixture-title');
    assert.equal(format.month(10), new Intl.DateTimeFormat('de', { month: 'short' }).format(new Date(2020, 9, 1)));
    assert.equal(home.rowQuery(saved[0].filter, library).genre, 'Horror');
  } finally { await i18n.changeLanguage('en'); }
});

test("the Watchlist's Home row sits where the Watchlist sits among the libraries", () => {
  const films = { id: 1, name: 'Films', kind: 'movies' };
  const series = { id: 2, name: 'Series', kind: 'shows' };
  const order = (at) => home.resolveRows([{ id: 'collection:c', title: 'Picks' }], [films, series], at).map(row => row.id).filter(id => id !== 'next-up' && id !== 'seasonal');
  assert.deepEqual(order(undefined), ['continue', 'watchlist', 'recent:1', 'recent:2', 'collection:c']);
  assert.deepEqual(order(1), ['continue', 'recent:1', 'watchlist', 'recent:2', 'collection:c']);
  assert.deepEqual(order(2), ['continue', 'recent:1', 'recent:2', 'watchlist', 'collection:c']);
  assert.deepEqual(order(9), ['continue', 'recent:1', 'recent:2', 'watchlist', 'collection:c']);
});
