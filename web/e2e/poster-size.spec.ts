import { test as base, expect, type Locator } from '@playwright/test';
import { copyFile, mkdir, readdir, rm } from 'node:fs/promises';
import { extname, join } from 'node:path';

const test = base.extend<{ library: { id: number; playback: string; empty: string } }>({
  library: async ({ page }, use) => {
    const reset = await fetch('http://127.0.0.1:18200/reset', { method: 'POST' });
    expect(reset.ok).toBeTruthy();
    const media = await reset.json();
    const errors: string[] = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.context().route('**/*', route => {
      const url = new URL(route.request().url());
      return url.hostname === '127.0.0.1' || url.protocol === 'data:' ? route.continue() : route.abort();
    });
    expect((await page.request.post('/api/auth/setup', { data: { name: 'Size owner', password: 'Poster-size-fixture-123', local_bypass: false, tmdb_api_key: 'fixture-not-a-secret' } })).ok()).toBeTruthy();
    expect((await page.request.patch('/api/settings', { data: { scan_automatic: false, scan_on_start: false, scan_interval_minutes: 0, preview_mode: 'off', detection_mode: 'off', analysis_enabled: false, metadata_refresh_enabled: false, auto_backups_enabled: false } })).ok()).toBeTruthy();
    expect((await page.request.patch('/api/me/prefs', { data: { show_home_hero: false } })).ok()).toBeTruthy();
    const created = await page.request.post('/api/libraries', { data: { name: 'Size library', kind: 'movies', paths: [media.movies] } });
    expect(created.ok()).toBeTruthy();
    const library = await created.json();
    await expect.poll(async () => (await (await page.request.get(`/api/libraries/${library.id}/scan`)).json()).running).toBe(false);
    await use({ ...library, playback: media.playback, empty: media.empty });
    expect(errors).toEqual([]);
  },
});
const width = (item: Locator) => item.evaluate(node => node.getBoundingClientRect().width);

test('poster sizing: Home menu, immediate drag preview, one save and independent Library size survive restart', async ({ page, library }, info) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Home', exact: true, level: 1 })).toBeVisible();
  await page.getByRole('button', { name: 'Home actions', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Edit Home', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'Edit Home', exact: true });
  await expect(dialog).toBeVisible();
  await dialog.getByRole('button', { name: 'Done', exact: true }).click();
  const slider = page.getByRole('slider', { name: 'Card size', exact: true });
  await expect(slider).toHaveValue('100');
  const card = page.getByRole('region', { name: 'Recently Added in Size library', exact: true }).locator('[data-item]').first();
  await expect.poll(() => width(card)).toBe(156);
  let saves = 0;
  page.on('request', request => { if (new URL(request.url()).pathname === '/api/me/prefs' && request.method() === 'PATCH') saves++; });
  const box = (await slider.boundingBox())!;
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width - 2, box.y + box.height / 2, { steps: 8 });
  await expect(slider).toHaveValue('150');
  await expect.poll(() => width(card)).toBe(234);
  expect(saves).toBe(0);
  await page.mouse.up();
  await expect.poll(async () => (await (await page.request.get('/api/me/prefs')).json()).home_poster_size).toBe(150);
  await expect(slider).toBeEnabled();
  expect(saves).toBe(1);
  await page.screenshot({ path: info.outputPath('home-size-desktop.png') });
  await page.goto(`/library/${library.id}`);
  await expect(slider).toHaveValue('100');
  const libraryCard = page.locator('main .poster-grid [data-item]').first();
  const original = await width(libraryCard);
  await slider.focus(); await slider.press('Home');
  await expect.poll(async () => (await (await page.request.get('/api/me/prefs')).json()).library_poster_size).toBe(75);
  await expect.poll(() => width(libraryCard)).toBeLessThan(original);
  await page.screenshot({ path: info.outputPath('library-size-desktop.png') });
  expect((await fetch('http://127.0.0.1:18200/restart', { method: 'POST' })).ok).toBeTruthy();
  await page.reload(); await expect(slider).toHaveValue('75');
  await page.goto('/'); await expect(slider).toHaveValue('150');
});

test('poster sizing: failed save rolls back the preview and offers Retry', async ({ page, library: _library }, info) => {
  let fail = true, saves = 0;
  await page.route('**/api/me/prefs', route => {
    if (route.request().method() !== 'PATCH') return route.continue();
    saves++;
    return fail ? route.fulfill({ status: 503, json: { error: 'Poster preference temporarily unavailable' } }) : route.continue();
  });
  await page.goto('/');
  const slider = page.getByRole('slider', { name: 'Card size', exact: true });
  const card = page.getByRole('region', { name: 'Recently Added in Size library', exact: true }).locator('[data-item]').first();
  const original = await width(card);
  await slider.focus(); await slider.press('End');
  await expect(page.getByRole('alert')).toContainText('Poster preference temporarily unavailable');
  await expect(slider).toHaveValue('100');
  await expect.poll(() => width(card)).toBe(original);
  expect((await (await page.request.get('/api/me/prefs')).json()).home_poster_size).toBeUndefined();
  await page.screenshot({ path: info.outputPath('poster-size-retry.png') });
  fail = false; await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(slider).toHaveValue('150');
  await expect(page.getByRole('alert')).toHaveCount(0);
  expect(saves).toBe(2);
});

test('poster sizing: narrow Library controls stay beside the menu and ordinary profiles have independent sliders', async ({ page, library }, info) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`/library/${library.id}`);
  const slider = page.getByRole('slider', { name: 'Card size', exact: true });
  const menu = page.locator('main').getByRole('button', { name: 'More for Size library', exact: true });
  await expect(slider).toBeVisible(); await expect(menu).toBeVisible();
  const s = (await slider.boundingBox())!, m = (await menu.boundingBox())!;
  expect(s.x + s.width).toBeLessThan(m.x);
  expect(Math.abs(s.y - m.y)).toBeLessThan(4);
  await slider.focus(); await slider.press('End');
  await expect(slider).toHaveValue('150');
  await expect(slider).toBeEnabled();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
  await page.screenshot({ path: info.outputPath('library-size-phone.png') });
  for (const tab of ['Recommended', 'Collections']) {
    await page.getByRole('tab', { name: tab, exact: true }).click();
    await expect(slider).toHaveValue('150');
    await expect(menu).toBeVisible();
  }
  const created = await page.request.post('/api/profiles', { data: { name: 'Size viewer', is_admin: false } });
  expect(created.ok()).toBeTruthy();
  const viewer = await created.json();
  expect((await page.request.post('/api/auth/profile', { data: { id: viewer.id } })).ok()).toBeTruthy();
  await page.reload();
  await expect(slider).toHaveValue('100');
  await expect(menu).toHaveCount(0);
  await slider.focus(); await slider.press('Home');
  await expect.poll(async () => (await (await page.request.get('/api/me/prefs')).json()).library_poster_size).toBe(75);
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Home', exact: true, level: 1 })).toBeVisible();
  await expect(slider).toHaveValue('100');
  await expect(page.getByRole('button', { name: 'Home actions', exact: true })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
  await page.screenshot({ path: info.outputPath('home-size-phone.png') });
});


test('poster sizing: unavailable Library preferences explain the disabled slider and recover with Retry', async ({ page, library }) => {
  expect((await page.request.patch('/api/me/prefs', { data: { library_poster_size: 140 } })).ok()).toBeTruthy();
  let fail = true;
  await page.route('**/api/me/prefs', route => fail && route.request().method() === 'GET'
    ? route.fulfill({ status: 503, json: { error: 'Profile preferences unavailable' } }) : route.continue());
  await page.goto(`/library/${library.id}`);
  const slider = page.getByRole('slider', { name: 'Card size', exact: true });
  await expect(slider).toBeDisabled();
  await expect(page.getByRole('alert')).toContainText('Profile preferences unavailable');
  fail = false;
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(slider).toBeEnabled();
  await expect(slider).toHaveValue('140');
  await expect(page.getByRole('alert')).toHaveCount(0);
});

test('card sizing: thumbnail and backdrop Home rows resize together and preserve their aspect ratios', async ({ page, library }, info) => {
  const source = (await (await page.request.get(`/api/items?library_id=${library.id}`)).json()).items[0];
  const episode = { ...source, kind: 'episode', parent_id: 'fixture-series', series_title: 'Size series', title: 'Size episode', season_number: 1, episode_number: 2 };
  // Explicit presentation responses isolate both row styles; real persistence is exercised above.
  await page.route('**/api/home/continue*', route => route.fulfill({ json: [episode] }));
  await page.route('**/api/home/next-up', route => route.fulfill({ json: [episode] }));
  expect((await page.request.patch('/api/me/prefs', { data: { split_up_next: true, home: [{ id: 'continue', style: 'thumb' }, { id: 'next-up', style: 'arc' }] } })).ok()).toBeTruthy();
  const slider = page.getByRole('slider', { name: 'Card size', exact: true });
  for (const viewport of [1440, 390]) {
    expect((await page.request.patch('/api/me/prefs', { data: { home_poster_size: 100 } })).ok()).toBeTruthy();
    await page.setViewportSize({ width: viewport, height: 1100 });
    await page.goto('/');
    const thumbnail = page.getByRole('region', { name: 'Continue Watching', exact: true }).locator('.thumbnail-row-card');
    const backdrop = page.getByRole('region', { name: 'Up Next', exact: true }).locator('.backdrop-row-card');
    const t = viewport === 1440 ? 300 : 260, b = viewport === 1440 ? 240 : 208;
    await expect.poll(() => width(thumbnail)).toBe(t);
    await expect.poll(() => width(backdrop)).toBe(b);
    await slider.focus(); await slider.press('End');
    await expect.poll(() => width(thumbnail)).toBe(t * 1.5);
    await expect.poll(() => width(backdrop)).toBe(b * 1.5);
    await expect(slider).toBeEnabled();
    for (const [card, ratio] of [[thumbnail, 16 / 9], [backdrop, 4 / 3]] as const) {
      const art = (await card.locator(':scope > button').first().boundingBox())!;
      expect(art.width / art.height).toBeCloseTo(ratio, 1);
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
    await page.screenshot({ path: info.outputPath(`all-card-styles-${viewport}.png`) });
  }
});

test('card sizing: Other videos thumbnail and folder grids follow the Library size', async ({ page, library }, info) => {
  const names = await readdir(library.playback);
  const source = names.find(name => /\.(mp4|mkv)$/.test(name));
  expect(source).toBeTruthy();
  const folder = join(library.empty, 'Size folder');
  await mkdir(folder, { recursive: true });
  await copyFile(join(library.playback, source!), join(folder, `Size clip${extname(source!)}`));
  try {
    const created = await page.request.post('/api/libraries', { data: { name: 'Size videos', kind: 'videos', paths: [library.empty] } });
    expect(created.ok()).toBeTruthy();
    const lib = await created.json();
    await expect.poll(async () => (await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json()).running).toBe(false);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`/library/${lib.id}`);
    const slider = page.getByRole('slider', { name: 'Card size', exact: true });
    const card = page.locator('.thumbnail-grid [data-item]').first();
    await expect(card).toBeVisible();
    const before = await width(card);
    await slider.focus(); await slider.press('End');
    await expect.poll(() => width(card)).toBeGreaterThan(before);
    await expect(slider).toBeEnabled();
    const artwork = card.locator('a > div').first();
    const box = (await artwork.boundingBox())!;
    expect(box.width / box.height).toBeCloseTo(16 / 9, 1);
    await page.screenshot({ path: info.outputPath('library-thumbnails.png') });
    await page.getByRole('tab', { name: 'Folders', exact: true }).click();
    const cover = page.locator('.video-folder-grid > a').first();
    await expect(cover).toBeVisible();
    const large = await width(cover);
    await slider.focus(); await slider.press('Home');
    await expect.poll(() => width(cover)).toBeLessThan(large);
    await page.screenshot({ path: info.outputPath('library-folder-covers.png') });
  } finally {
    await rm(folder, { recursive: true, force: true });
  }
});

test('collection sizing: opening a collection inherits Library size, persists and leaves hero/text unchanged', async ({ page, library }, info) => {
  const items = (await (await page.request.get(`/api/items?library_id=${library.id}`)).json()).items;
  const response = await page.request.post('/api/collections', { data: { library_id: library.id, kind: 'manual', title: 'Inherited size collection', item_ids: items.map((item: { id: string }) => item.id) } });
  expect(response.ok()).toBeTruthy();
  const collection = await response.json();
  expect((await page.request.patch('/api/me/prefs', { data: { home_poster_size: 150 } })).ok()).toBeTruthy();
  await page.setViewportSize({ width: 1440, height: 1000 });
  const openFromLibrary = async (key: 'Home' | 'End') => {
    await page.goto(`/library/${library.id}`);
    const slider = page.getByRole('slider', { name: 'Card size', exact: true });
    await expect(slider).toBeEnabled();
    await slider.focus(); await slider.press(key);
    await expect(slider).toBeEnabled();
    await page.getByRole('tab', { name: 'Collections', exact: true }).click();
    await page.locator(`main a[href="/collection/${collection.id}"]`).click();
    await expect(page.getByRole('heading', { name: collection.title, exact: true, level: 1 })).toBeVisible();
  };
  await openFromLibrary('End');
  const card = page.locator('.poster-grid [data-item]').first();
  await expect(card).toBeVisible();
  const large = await width(card);
  const hero = page.locator('main .w-44');
  const heroWidth = await width(hero);
  const heading = page.getByRole('heading', { name: collection.title, exact: true, level: 1 });
  const fontSize = await heading.evaluate(node => getComputedStyle(node).fontSize);
  await page.screenshot({ path: info.outputPath('collection-size-desktop.png'), fullPage: true });
  await openFromLibrary('Home');
  await expect.poll(() => width(card)).toBeLessThan(large);
  const small = await width(card);
  expect(await width(hero)).toBe(heroWidth);
  expect(await heading.evaluate(node => getComputedStyle(node).fontSize)).toBe(fontSize);
  expect((await (await page.request.get('/api/me/prefs')).json()).home_poster_size).toBe(150);
  await page.reload();
  await expect.poll(() => width(card)).toBe(small);
  await page.setViewportSize({ width: 390, height: 844 });
  await card.scrollIntoViewIfNeeded();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
  await page.screenshot({ path: info.outputPath('collection-size-phone.png'), fullPage: true });
  const viewer = await (await page.request.post('/api/profiles', { data: { name: 'Collection size viewer', is_admin: false } })).json();
  expect((await page.request.post('/api/auth/profile', { data: { id: viewer.id } })).ok()).toBeTruthy();
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.reload();
  await expect.poll(() => width(card)).toBeGreaterThan(small);
  expect(await width(card)).toBeLessThan(large);
});

test('collection sizing: failed preference loading keeps the collection usable and Retry restores its saved size', async ({ page, library }) => {
  const items = (await (await page.request.get(`/api/items?library_id=${library.id}`)).json()).items;
  const response = await page.request.post('/api/collections', { data: { library_id: library.id, kind: 'manual', title: 'Size recovery collection', item_ids: items.map((item: { id: string }) => item.id) } });
  expect(response.ok()).toBeTruthy();
  const collection = await response.json();
  expect((await page.request.patch('/api/me/prefs', { data: { library_poster_size: 125 } })).ok()).toBeTruthy();
  let fail = true;
  await page.route('**/api/me/prefs', route => fail && route.request().method() === 'GET'
    ? route.fulfill({ status: 503, json: { error: 'Collection preferences temporarily unavailable' } }) : route.continue());
  await page.goto(`/collection/${collection.id}`);
  await expect(page.getByRole('heading', { name: collection.title, exact: true, level: 1 })).toBeVisible();
  await expect(page.getByRole('alert')).toContainText('Collection preferences temporarily unavailable');
  const card = page.locator('.poster-grid [data-item]').first();
  const fallback = await width(card);
  fail = false;
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByRole('alert')).toHaveCount(0);
  await expect.poll(() => width(card)).toBeGreaterThan(fallback);
});
