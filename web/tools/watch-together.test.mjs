// Watch Together's client maths (src/lib/watch-together.ts): invite codes, clock offset,
// where the room should be, and how a drifting player is brought back (docs/WATCH-TOGETHER-API.md).
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import ts from 'typescript';

const source = readFileSync(new URL('../src/lib/watch-together.ts', import.meta.url), 'utf8');
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText;
const mod = {};
new Function('require', 'exports', compiled)(() => ({}), mod);
const { normalizeCode, formatCode, clockOffset, expectedPosition, correction } = mod;

test('invite codes are typed any way and shown as XXXX-XXXX', () => {
  assert.equal(normalizeCode(' abcd-12 34 '), 'ABCD1234');
  assert.equal(formatCode('abcd1234'), 'ABCD-1234');
  assert.equal(formatCode('ABC'), 'ABC');
});

test('the clock offset comes from the sample with the shortest round trip', () => {
  // Server 1000 ms ahead. A slow, lopsided sample would suggest otherwise; the fast one wins.
  const slow = { c0: 0, s1: 1900, s2: 1900, c3: 1000 }; // 1000 ms round trip, asymmetric
  const fast = { c0: 5000, s1: 6010, s2: 6010, c3: 5020 }; // 20 ms round trip
  assert.equal(clockOffset([slow, fast]), 1000);
  assert.equal(clockOffset([]), null);
});

test('a playing room moves on with the server clock; a paused or waiting one stays put', () => {
  const room = { state: 'playing', position_ms: 60_000, at_ms: 1_000_000, rate: 1 };
  assert.equal(expectedPosition(room, 1_005_000), 65_000);
  assert.equal(expectedPosition({ ...room, state: 'paused' }, 1_005_000), 60_000);
  assert.equal(expectedPosition({ ...room, state: 'waiting' }, 1_005_000), 60_000);
  // A server time slightly before at_ms (clock estimates) never goes backwards.
  assert.equal(expectedPosition(room, 999_000), 60_000);
});

test('small drift is left alone, middling drift nudges the speed, large drift jumps', () => {
  assert.deepEqual(correction(10_400, 10_000), { kind: 'none' });
  assert.deepEqual(correction(12_000, 10_000), { kind: 'rate', rate: 0.97 }); // ahead: slow down
  assert.deepEqual(correction(8_000, 10_000), { kind: 'rate', rate: 1.03 }); // behind: speed up
  assert.deepEqual(correction(20_000, 10_000), { kind: 'seek', to: 10_000 });
});
