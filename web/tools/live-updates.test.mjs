// Live updates (src/lib/live-updates.ts): which screens each hint refreshes, and how hints are
// batched so a scan's steady stream of hints refreshes the page every few seconds, not constantly.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import ts from 'typescript';

// Loaded as the other web tests load source: transpiled, with React's modules stubbed.
const source = readFileSync(new URL('../src/lib/live-updates.ts', import.meta.url), 'utf8');
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText;
const mod = {};
new Function('require', 'exports', compiled)(() => ({}), mod);
const { HintBatcher, prefixesFor, topicQueries } = mod;

test('topics map to the queries they refresh; resync means everything; unknown topics nothing', () => {
  const p = prefixesFor(['catalog', 'artwork']);
  for (const key of ['items', 'continue', 'featured', 'artwork']) assert.ok(p.has(key), key);
  assert.ok(!p.has('auth'));
  assert.equal(prefixesFor(['catalog', 'resync']), null);
  assert.equal(prefixesFor(['some-future-topic']).size, 0);
  // Every topic the server documents is handled (docs/EVENTS-API.md).
  for (const t of ['catalog', 'library', 'profile', 'playback', 'sessions', 'activity', 'artwork', 'optimized', 'previews', 'markers', 'maintenance'])
    assert.ok(topicQueries[t]?.length, t);
});

test('a steady stream of hints refreshes at most every five seconds, never not at all', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout', 'Date'] });
  const flushed = [];
  const b = new HintBatcher((topics) => flushed.push([Date.now(), [...topics].sort()]), 1_500, 5_000, () => Date.now());
  // Two hints a second for twenty seconds, as during a scan.
  for (let ms = 0; ms < 20_000; ms += 500) {
    b.add(ms % 1000 ? 'artwork' : 'catalog');
    t.mock.timers.tick(500);
  }
  t.mock.timers.tick(10_000);
  const times = flushed.map(([at]) => at);
  assert.equal(times[0], 1_500, 'first refresh soon after the first hint');
  for (let i = 1; i < times.length; i++) assert.ok(times[i] - times[i - 1] >= 5_000, `gap ${times[i] - times[i - 1]}`);
  assert.ok(times.length >= 4 && times.length <= 5, `${times.length} refreshes in 20 s`);
  assert.deepEqual(flushed[0][1], ['artwork', 'catalog']);
  b.stop();
});

test('a single hint refreshes after a short pause, and stop drops what was waiting', (t) => {
  t.mock.timers.enable({ apis: ['setTimeout', 'Date'] });
  const flushed = [];
  const b = new HintBatcher((topics) => flushed.push([...topics]), 1_500, 5_000, () => Date.now());
  b.add('profile');
  t.mock.timers.tick(1_499);
  assert.equal(flushed.length, 0);
  t.mock.timers.tick(1);
  assert.deepEqual(flushed, [['profile']]);
  b.add('catalog');
  b.stop();
  t.mock.timers.tick(10_000);
  assert.equal(flushed.length, 1);
});
