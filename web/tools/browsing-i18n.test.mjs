import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import ts from 'typescript';
import { createInstance } from 'i18next';

const i18n = createInstance();
const catalog = JSON.parse(readFileSync(new URL('../src/locales/en/library.json', import.meta.url), 'utf8'));
await i18n.init({ lng: 'en', fallbackLng: 'en', resources: { en: { library: catalog } } });
const source = readFileSync(new URL('../src/lib/browsing-labels.ts', import.meta.url), 'utf8');
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText;
const labels = {};
new Function('require', 'exports', compiled)(() => ({ default: i18n }), labels);

test('browsing noun counts retain English zero, singular and plural forms', () => {
  for (const unit of ['movie', 'show', 'video', 'title']) {
    for (const count of [0, 1, 2, 500]) {
      assert.equal(labels.countUnit(count, unit), `${count} ${unit}${count === 1 ? '' : 's'}`);
    }
  }
});
test('browsing unit identifiers remain stable when translated labels change', async () => {
  i18n.addResourceBundle('de', 'library', { counts: { shows_one: '{{count}} fixture-show', shows_other: '{{count}} fixture-shows' } });
  await i18n.changeLanguage('de');
  try {
    assert.equal(labels.countUnit(1, 'show'), '1 fixture-show');
    assert.equal(labels.countUnit(2, 'show'), '2 fixture-shows');
    assert.equal(labels.countUnit(2, 'movie'), '2 movies');
  } finally { await i18n.changeLanguage('en'); }
});
