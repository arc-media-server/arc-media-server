import { test, expect, type Page, type TestInfo } from '@playwright/test';
import type { Item, Library } from '../src/lib/api';
type Fixtures = { movies: string; shows: string; empty: string; playback: string };
const password = 'Arc-browser-test-123';
let media: Fixtures;

test('review: failed home dependencies resemble empty content', async ({ page }, info) => {
  await seed(page);
  let failures = 0;
  await page.route('**/api/libraries', route => { failures++; return route.fulfill({ status: 503, json: { error: 'Audit library service unavailable' } }); });
  await page.goto('/');
  await expect.poll(() => failures).toBeGreaterThanOrEqual(2);
  await expect(page.getByRole('heading', { name: 'Welcome to Arc', exact: true })).toBeVisible();
  await expect(page.getByText('Audit library service unavailable')).toHaveCount(0);
  await page.screenshot({ path: info.outputPath('home-failed-library.png') });
  await note(info, 'home-library-error', { existingLibrary: true, requestStatus: 503, visibleState: 'Welcome to Arc / Add a Library', errorVisible: false });
  await page.unroute('**/api/libraries');
  await page.route('**/api/home/continue*', route => route.fulfill({ status: 503, json: { error: 'Audit continue unavailable' } }));
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Recently Added in Audit Movies', exact: true })).toBeVisible();
  await page.waitForTimeout(1500);
  await expect(page.getByRole('heading', { name: 'Continue Watching', exact: true })).toHaveCount(0);
  await expect(page.getByText('Audit continue unavailable')).toHaveCount(0);
  await note(info, 'home-row-error', { requestStatus: 503, continueRowVisible: false, errorVisible: false });
});

test('review: preference and episode-order failures have no feedback', async ({ page }, info) => {
  const { item } = await seed(page, true);
  await page.route('**/api/me/prefs', route => route.request().method() === 'PATCH' ? route.fulfill({ status: 503, json: { error: 'Audit preference save failed' } }) : route.continue());
  await page.goto('/settings');
  const spoiler = page.getByRole('switch', { name: 'Hide spoilers', exact: true });
  await expect(spoiler).not.toBeChecked();
  const save = page.waitForResponse(r => r.url().endsWith('/api/me/prefs') && r.request().method() === 'PATCH');
  await spoiler.click(); expect((await save).status()).toBe(503);
  await expect(spoiler).not.toBeChecked();
  await expect(page.getByText('Audit preference save failed')).toHaveCount(0);
  await note(info, 'profile-save-error', { requestStatus: 503, rolledBack: true, errorVisible: false });
  await page.route('**/display-ordering', route => route.fulfill({ status: 503, json: { error: 'Audit ordering save failed' } }));
  await page.goto('/series/' + item.id);
  await page.getByRole('button', { name: /Aired Order/ }).click();
  const change = page.waitForResponse(r => r.url().endsWith('/display-ordering'));
  await page.getByRole('menuitemradio', { name: /Story Order/ }).click();
  expect((await change).status()).toBe(503);
  await expect(page.getByRole('button', { name: /Aired Order/ })).toBeEnabled();
  await expect(page.getByText('Audit ordering save failed')).toHaveCount(0);
  await note(info, 'order-save-error', { requestStatus: 503, retainedOrder: 'Aired Order', errorVisible: false });
  await page.screenshot({ path: info.outputPath('order-failed-save.png') });
});

test('review: mobile playback and administration geometry', async ({ page }, info) => {
  const { item } = await seed(page);
  await page.request.post('/api/profiles', { data: { name: 'Audit Household', color: 'blue' } });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/profiles');
  await expect(page.getByRole('button', { name: 'Add Profile', exact: true })).toBeVisible();
  const profiles = await page.evaluate(() => ({ viewport: innerWidth, scrollWidth: document.documentElement.scrollWidth }));
  await page.screenshot({ path: info.outputPath('profiles-390.png'), fullPage: true });
  await page.goto('/settings');
  await expect(page.getByRole('heading', { name: 'Library scanning', exact: true })).toBeVisible();
  const settings = await page.evaluate(() => ({ viewportHeight: innerHeight, documentHeight: document.documentElement.scrollHeight, sections: Array.from(document.querySelectorAll('main h2')).map(e => e.textContent) }));
  await page.goto('/movie/' + item.id);
  await page.getByRole('button', { name: 'Play', exact: true }).click();
  await expect(page.locator('video')).toBeVisible();
  await page.getByRole('button', { name: 'Quality', exact: true }).waitFor();
  await expect.poll(() => page.locator('video').evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(0);
  // Pause keeps controls visible while measuring their actual viewport coordinates.
  await page.keyboard.press('Space');
  const controls = await page.getByRole('button').evaluateAll(bs => bs.map(b => ({ label: b.getAttribute('aria-label'), r: b.getBoundingClientRect() })).filter(b => b.label && /Quality|Subtitles|Speed|Fullscreen|Pause|Play \(/.test(b.label)).map(b => ({ label: b.label, left: b.r.left, right: b.r.right, top: b.r.top, width: b.r.width, height: b.r.height })));
  await page.screenshot({ path: info.outputPath('player-390.png') });
  await note(info, 'mobile-geometry', { profiles, settings, controls, offscreen: controls.filter(b => b.left < 0 || b.right > 390) });
  await page.getByRole('button', { name: 'Close player', exact: true }).click();
});

test('review: custom row has no full-results route and reordering requires pointer', async ({ page }, info) => {
  const { lib, item } = await seed(page);
  const patch = { home: [{ id: 'custom:audit', title: 'Audit short movies', filter: { library_id: lib.id, max_runtime: 90, sort: 'added' } }] };
  expect((await page.request.patch('/api/me/prefs', { data: patch })).ok()).toBe(true);
  let requestedLimit = '';
  await page.route('**/api/items?*', route => {
    const u = new URL(route.request().url());
    if (u.searchParams.get('max_runtime') !== '90') return route.continue();
    requestedLimit = u.searchParams.get('limit') || '';
    return route.fulfill({ json: { total: 60, items: Array.from({ length: 24 }, (_, i) => ({ ...item, id: 'audit-' + i, title: 'Audit ' + i })) } });
  });
  await page.goto('/');
  const heading = page.getByRole('heading', { name: 'Audit short movies', exact: true });
  await expect(heading).toBeVisible();
  expect(requestedLimit).toBe('24');
  await expect(heading.getByRole('link')).toHaveCount(0);
  const row = heading.locator('xpath=ancestor::section');
  await expect(row.getByRole('link')).toHaveCount(24);
  await expect(row.getByRole('button', { name: /more|all/i })).toHaveCount(0);
  await page.getByRole('button', { name: 'Edit Home', exact: true }).first().click();
  const dialog = page.getByRole('dialog');
  const handles = dialog.getByRole('button', { name: /^Drag / });
  const names = await handles.evaluateAll(bs => bs.map(b => b.getAttribute('aria-label')));
  await handles.first().focus();
  await page.keyboard.press('Space'); await page.keyboard.press('ArrowDown'); await page.keyboard.press('Space');
  const afterNames = await handles.evaluateAll(bs => bs.map(b => b.getAttribute('aria-label')));
  expect(afterNames).toEqual(names);
  await expect(dialog.getByRole('button', { name: /Move .*up|Move .*down/i })).toHaveCount(0);
  await page.screenshot({ path: info.outputPath('home-editor.png') });
  await note(info, 'home-customisation', { requestedLimit, totalMatches: 60, shown: 24, fullResultsLink: false, keyboardOrderUnchanged: true, handles: names });
});
test.beforeEach(async ({ page }) => {
  const r = await fetch('http://127.0.0.1:18200/reset', { method: 'POST' });
  if (!r.ok) throw new Error(await r.text());
  media = await r.json();
  await page.context().route('**/*', route => { const u = new URL(route.request().url()); return u.hostname === '127.0.0.1' || u.protocol === 'data:' ? route.continue() : route.abort(); });
  await setup(page);
});
async function seed(page: Page, shows = false) {
  const r = await page.request.post('/api/libraries', { data: { name: shows ? 'Audit TV' : 'Audit Movies', kind: shows ? 'shows' : 'movies', paths: [shows ? media.shows : media.movies] } });
  expect(r.ok()).toBe(true); const lib: Library = await r.json();
  await expect.poll(async () => (await (await page.request.get('/api/libraries/' + lib.id + '/scan')).json()).running, { timeout: 30000 }).toBe(false);
  const items: Item[] = (await (await page.request.get('/api/items?library_id=' + lib.id)).json()).items;
  return { lib, item: items.find((i: Item) => i.title === (shows ? 'Arc Show' : 'Arc Movie'))!, items };
}
async function note(info: TestInfo, name: string, value: unknown) { console.log('AUDIT', name, JSON.stringify(value)); await info.attach(name, { body: JSON.stringify(value, null, 2), contentType: 'application/json' }); }
async function setup(page: Page) {
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Welcome to Arc' })).toBeVisible();
  await page.getByLabel('Your name', { exact: true }).fill('Admin');
  await page.getByLabel('Password', { exact: true }).fill(password);
  await page.getByLabel('Password again', { exact: true }).fill(password);
  await page.getByRole('switch', { name: 'Skip the password at home' }).uncheck();
  await page.getByRole('button', { name: 'Continue', exact: true }).click();
  await page.getByRole('textbox', { name: /^TMDB API key/ }).fill('fixture-key-not-a-secret');
  await page.getByRole('button', { name: 'Finish', exact: true }).click();
  await expect(page.getByRole('dialog', { name: 'Add a Library', exact: true })).toBeVisible();
  await page.getByRole('dialog').getByRole('button', { name: 'Cancel', exact: true }).last().click();
  await expect(page.getByRole('heading', { name: 'Libraries', exact: true })).toBeVisible();
}
