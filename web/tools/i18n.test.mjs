import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readdirSync, readFileSync } from 'node:fs';
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

test('keyboard tokens stay separate from translated navigation labels', () => {
  const resources = { common: { home: 'Home' } };
  const translated = audit(resources, { 'test.tsx': "const { t } = useTranslation('common'); if (event.key === t('home')) go();" });
  assert.match(translated.join('\n'), /keyboard tokens must not be translated/);
  assert.deepEqual(audit(resources, { 'test.tsx': "const { t } = useTranslation('common'); const label = t('home'); if (event.key === 'Home') go();" }), []);
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
  assert.equal(f.fixed(2.55, 1), '2.5');
  const stamp = new Date('2020-01-02T15:04:05Z');
  assert.equal(f.localDate(stamp), stamp.toLocaleDateString());
  assert.equal(f.localTime(stamp), stamp.toLocaleTimeString());
  assert.equal(f.localDateTime(stamp), stamp.toLocaleString());
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
  assert.equal(exports.fixed(1234.5, 1), '1234,5');
  const stamp = new Date('2020-01-02T15:04:05Z');
  assert.equal(exports.localDateTime(stamp), stamp.toLocaleString('de'));
  assert.equal(exports.localDate(stamp), stamp.toLocaleDateString('de'));
  assert.equal(exports.localTime(stamp), stamp.toLocaleTimeString('de'));
  assert.equal(exports.month(1, 'long'), 'Januar');
  assert.equal(exports.weekday(0), new Intl.DateTimeFormat('de', { weekday: 'short', timeZone: 'UTC' }).format(new Date(Date.UTC(2020, 5, 7))));
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

test('audit rejects run-together compound keys but accepts genuine single words', () => {
  assert.match(audit({ common: { accessdevices: 'Access' } }, { 'test.tsx': "t('accessdevices')" }).join('\n'), /use camelCase/);
  assert.deepEqual(audit({ common: { notifications: 'Notifications', accessDevices: 'Access' } }, { 'test.tsx': "t('notifications'); t('accessDevices')" }), []);
});
test('counted words require plural pairs while attached unit abbreviations stay single', () => {
  assert.match(audit({ common: { files: '{{count}} files' } }, { 'test.tsx': "t('files', {count: 2})" }).join('\n'), /require _one and _other/);
  assert.deepEqual(audit({ common: { minutes: '{{count}}m', hours: '{{count}}h', seconds: '{{count}}s', channels: '{{count}}ch' } }, { 'test.tsx': "t('minutes'); t('hours'); t('seconds'); t('channels')" }), []);
});
test('language names reuse a DisplayNames instance for each locale', async () => {
  const Original = Intl.DisplayNames;
  let constructions = 0;
  Intl.DisplayNames = class extends Original { constructor(...args) { super(...args); constructions++; } };
  const fresh = {};
  try {
    new Function('require', 'exports', compiled)(() => ({ default: i18n }), fresh);
    await i18n.changeLanguage('en');
    assert.equal(fresh.language('fr'), 'French');
    assert.equal(fresh.language('de'), 'German');
    assert.equal(constructions, 1);
    await i18n.changeLanguage('de');
    assert.equal(fresh.language('fr'), 'Französisch');
    assert.equal(constructions, 2);
    await i18n.changeLanguage('en');
    assert.equal(fresh.language('fr'), 'French');
    assert.equal(constructions, 2);
  } finally { Intl.DisplayNames = Original; await i18n.changeLanguage('en'); }
});

// Translators start from the English "one" form, and in some languages "one" also covers 21, 31…
// So a "_one" must be genuinely singular even where English never shows a count of 1: one that is
// word-for-word its "_other" and counts a plural noun ("{{count}} days ago") is a copy, not a form.
test('singular plural forms are singular', () => {
  const copies = [];
  for (const file of readdirSync(root + 'locales/en')) {
    const walk = (node, path) => {
      for (const [key, value] of Object.entries(node)) {
        if (typeof value !== 'string') { walk(value, `${path}${key}.`); continue; }
        if (!key.endsWith('_one') || node[key.replace(/_one$/, '_other')] !== value) continue;
        // The first two words after the count (past any rich-text tag), e.g. "{{count}} <b>days</b>".
        const after = value.split('{{count}}')[1]?.replace(/<[^>]+>/g, '').trim().split(/\s+/).slice(0, 2) ?? [];
        if (after.some(word => /^[a-z]{3,}s[.,]?$/i.test(word))) copies.push(`${file} ${path}${key}: ${value}`);
      }
    };
    walk(JSON.parse(readFileSync(root + 'locales/en/' + file, 'utf8')), '');
  }
  assert.deepEqual(copies, []);
});
