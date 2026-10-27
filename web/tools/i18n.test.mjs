import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import ts from 'typescript';
import { createInstance } from 'i18next';
import { audit, auditProject } from './i18n-audit.mjs';

const root = fileURLToPath(new URL('../src/', import.meta.url));
test('every used key exists and every English key is used', () => {
  assert.deepEqual(auditProject(root), []);
});
test('audit reports missing and unused keys', () => {
  assert.deepEqual(audit({ common: { actions: { retry: 'Retry' } } }, { 'test.tsx': "t('actions.missing')" }), ['test.tsx: missing English key common:actions.missing', 'Unused English key common:actions.retry']);
});
test('audit understands namespaces, hook aliases, rich text and plural families', () => {
  assert.deepEqual(audit({ common: { retry: 'Retry' }, auth: { heading: 'Welcome', items_one: '{{count}} item', items_other: '{{count}} items', help: 'Read <link>help</link>' } }, { 'test.tsx': `const { t: tr } = useTranslation(['auth', 'common']); tr('heading'); tr('items', {count: 2}); tr('common:retry'); <Trans ns="auth" i18nKey="help" components={{link: <a/>}} />` }), []);
});
test('audit rejects dynamic keys instead of silently missing them', () => {
  assert.match(audit({}, { 'test.tsx': 't(variable)' }).join('\n'), /keys must be static/);
});

// Exercise the real formatting module with a private i18next instance. No Vite/browser or
// production translation is needed; the synthetic second language stays inside this test.
const i18n = createInstance();
await i18n.init({ lng: 'en', fallbackLng: 'en', resources: { en: { common: JSON.parse(readFileSync(root + 'locales/en/common.json', 'utf8')) } }, interpolation: { escapeValue: false } });
const compiled = ts.transpileModule(readFileSync(root + 'lib/format.ts', 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText;
const exports = {};
new Function('require', 'exports', compiled)(name => { assert.equal(name, '../i18n'); return { default: i18n }; }, exports);

test('English formatting retains dates, units, plurals, rounding and source names', () => {
  const f = exports;
  assert.equal(f.formatLocale(), 'en-GB');
  assert.equal(f.date('2020-01-02'), '2 Jan 2020');
  assert.equal(f.date('bad'), 'bad');
  assert.equal(f.date(), '');
  assert.equal(f.runtime(60), '1h');
  assert.equal(f.runtime(64), '1h 4m');
  assert.equal(f.clock(3_849_000), '1:04:09');
  assert.equal(f.fileSize(2.55e9), '2.5 GB');
  assert.equal(f.fileSize(20e6), '20 MB');
  assert.equal(f.watchTime(100 * 3_600_000), '100 hours');
  assert.equal(f.channels(2), 'Stereo');
  assert.equal(f.language('fr'), 'French');
  assert.equal(f.plural(1, 'file'), '1 file');
  assert.equal(f.plural(2, 'file'), '2 files');
  assert.equal(f.plural(2, 'person', 'people'), '2 people');
  assert.equal(f.scanProgress({ files_seen: 1, matched: 0 }), 'Looking through folders · 1 file');
  assert.equal(f.scanProgress({ files_seen: 2, matched: 1200, phase: 'Matching', total: 1812, done: 120 }), 'Matching · 120 of 1,812 · 1,200 identified');
  assert.equal(f.ago(Date.now() / 1000 - 86400), 'yesterday');
  assert.equal(f.ago(Date.now() / 1000 - 40 * 86400), 'a month ago');
});
test('format helpers follow the active language and retain English fallback', async () => {
  i18n.addResourceBundle('de', 'common', { format: { mono: 'test-mono' } });
  await i18n.changeLanguage('de');
  assert.equal(exports.number(1234.5), '1.234,5');
  assert.equal(exports.date('2020-01-02'), new Intl.DateTimeFormat('de', { day: 'numeric', month: 'short', year: 'numeric' }).format(new Date('2020-01-02T00:00:00')));
  assert.equal(exports.language('fr'), 'Französisch');
  assert.equal(exports.channels(1), 'test-mono');
  assert.equal(exports.channels(2), 'Stereo');
  await i18n.changeLanguage('en');
});

async function setupLanguage(browserLanguages, extraModules = {}) {
  const detector = (await import('i18next-browser-languagedetector')).default;
  const instance = createInstance();
  const originalNavigator = Object.getOwnPropertyDescriptor(globalThis, 'navigator');
  Object.defineProperty(globalThis, 'navigator', { configurable: true, value: { languages: browserLanguages, language: browserLanguages[0] } });
  const modules = Object.fromEntries(['common', 'auth'].map(ns => [`./locales/en/${ns}.json`, { default: JSON.parse(readFileSync(root + `locales/en/${ns}.json`, 'utf8')) }]));
  const source = readFileSync(root + 'i18n.ts', 'utf8').replaceAll('import.meta.glob', 'glob');
  const js = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText;
  const result = {};
  const requests = [];
  try {
    new Function('require', 'exports', 'glob', js)(name => {
      if (name === 'i18next') return { default: instance };
      if (name === 'i18next-browser-languagedetector') return { default: detector };
      if (name === 'react-i18next') return { initReactI18next: { type: '3rdParty', init() {} } };
      throw new Error(name);
    }, result, (_pattern, options) => options?.eager ? modules : {
      ...Object.fromEntries(Object.entries(modules).map(([key, module]) => [key, async () => { requests.push(key); return module; }])),
      ...Object.fromEntries(Object.entries(extraModules).map(([key, load]) => [key, async () => { requests.push(key); return load(); }])),
    });
  } finally {
    if (originalNavigator) Object.defineProperty(globalThis, 'navigator', originalNavigator);
    else delete globalThis.navigator;
  }
  return { instance, ready: result.languageReady, requests };
}
test('unsupported browser languages keep bundled English and load no chunks', async () => {
  const { instance, ready, requests } = await setupLanguage(['fr-FR', 'es']);
  await ready;
  assert.equal(instance.t('auth:setup.heading'), 'Welcome to Arc');
  assert.equal(instance.resolvedLanguage, 'en');
  assert.deepEqual(requests, []);
});
test('an available language loads on demand while English stays usable, including missing namespaces', async () => {
  let release;
  const loading = new Promise(resolve => { release = resolve; });
  const { instance, ready, requests } = await setupLanguage(['fr-FR'], { './locales/fr/common.json': () => loading });
  assert.equal(instance.isInitialized, true);
  assert.equal(instance.t('auth:setup.heading'), 'Welcome to Arc');
  assert.equal(instance.t('navigation.home'), 'Home');
  release({ default: { navigation: { home: 'synthetic translated Home' } } });
  await ready;
  assert.equal(instance.resolvedLanguage, 'fr');
  assert.equal(instance.t('navigation.home'), 'synthetic translated Home');
  assert.equal(instance.t('auth:setup.heading'), 'Welcome to Arc');
  assert.deepEqual(requests, ['./locales/fr/common.json']);
});
test('a failed locale chunk leaves usable English text', async () => {
  const { instance, ready } = await setupLanguage(['fr'], { './locales/fr/common.json': async () => { throw new Error('fixture offline'); } });
  await ready;
  assert.equal(instance.t('navigation.home'), 'Home');
  assert.equal(instance.t('auth:setup.heading'), 'Welcome to Arc');
});
test('hook namespaces are scoped to each component', () => {
  assert.deepEqual(audit({ auth: { heading: 'Welcome' }, common: { heading: 'Home' } }, { 'test.tsx': `function Auth() { const { t } = useTranslation('auth'); return t('heading'); } function Home() { const { t } = useTranslation('common'); return t('heading'); }` }), []);
});
test('audit rejects an incomplete English plural family', () => {
  assert.deepEqual(audit({ common: { items_one: '{{count}} item' } }, { 'test.tsx': "t('items', { count: 2 })" }), ['test.tsx: missing English plural key common:items_other']);
});
