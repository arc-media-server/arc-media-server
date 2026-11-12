import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import ts from 'typescript';

function load(name, dependencies = {}) {
  const source = readFileSync(new URL(`../src/lib/${name}.ts`, import.meta.url), 'utf8');
  const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText;
  const exports = {};
  new Function('require', 'exports', compiled)((id) => {
    if (id in dependencies) return dependencies[id];
    throw new Error(`Unexpected dependency ${id}`);
  }, exports);
  return exports;
}
const filters = load('item-filters');
const home = load('home', { './item-filters': filters, '@/i18n': { default: { t: key => key } }, '@tanstack/react-query': {}, '@/lib/api': {} });
const browse = load('browse-state', { './item-filters': filters });

test('saved person and unwatched filters survive Home query, library link and remembered state', () => {
  const input = { library_id: 7, person_id: 42, person_role: 'actor', watch_state: 'unwatched', video_codec: 'hevc', audio_codec: 'truehd', dynamic_range: 'hdr10', sort: 'added' };
  const query = home.rowQuery(input, { kind: 'movies' });
  const params = new URL(home.rowHref(input), 'https://fixture.invalid').searchParams;
  const parsed = filters.readItemFilters(params);
  assert.equal(query.person_id, 42);
  assert.equal(query.person_role, 'actor');
  assert.equal(query.unwatched, true);
  assert.equal(query.watch_state, undefined);
  assert.deepEqual(parsed, filters.pickItemFilters(query));
  const remembered = browse.persistentBrowseParams(params);
  assert.deepEqual(filters.readItemFilters(remembered), parsed);
  assert.equal(remembered.get('sort'), 'added');
});

test('all supported saved-row filters round-trip, preserving false/absent semantics', () => {
  const input = { genre: 'Drama', language: 'en', label: 'Favourite', decade: 1990, resolution: '4k', audio_channels: 8, added_within: 30, min_runtime: 10, max_runtime: 90, min_rating: 7.5, person_id: 5, person_role: 'director', watch_state: 'unstarted', unmatched: true };
  assert.deepEqual(filters.readItemFilters(filters.encodeItemFilters(input)), input);
  assert.deepEqual(filters.readItemFilters(new URLSearchParams()), {});
});

test('invalid stored controls are ignored and legacy Unwatched takes precedence', () => {
  const params = new URLSearchParams('sort=bad&reverse=1&person_id=-1&person_role=actor&resolution=no&dynamic_range=bogus&audio_channels=NaN&watch_state=played&unwatched=1&offset=99&token=private');
  const saved = browse.persistentBrowseParams(params);
  assert.equal(saved.toString(), 'unwatched=1&sort=title&reverse=1');
});
