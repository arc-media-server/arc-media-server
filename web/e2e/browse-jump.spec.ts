import { test as base, expect, type Page } from '@playwright/test';
import type { Item } from '../src/lib/api';

type Media = { movies: string; shows: string; empty: string };
const password = 'Arc-recovery-fixture-123';
const test = base.extend<{ media: Media }>({
  media: async ({ page }, use) => {
    const reset = await fetch('http://127.0.0.1:18200/reset', { method: 'POST' });
    if (!reset.ok) throw new Error(await reset.text());
    const errors: string[] = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.context().route('**/*', route => {
      const url = new URL(route.request().url());
      return url.hostname === '127.0.0.1' || url.protocol === 'data:' ? route.continue() : route.abort();
    });
    expect((await page.request.post('/api/auth/setup', { data: {
      name: 'Admin', password, local_bypass: false, tmdb_api_key: 'fixture-key-not-a-secret', region: 'GB',
    } })).ok()).toBeTruthy();
    expect((await page.request.patch('/api/settings', { data: {
      scan_automatic: false, scan_on_start: false, scan_interval_minutes: 0,
      preview_mode: 'off', detection_mode: 'off', analysis_enabled: false,
      metadata_refresh_enabled: false, auto_backups_enabled: false,
    } })).ok()).toBeTruthy();
    await use(await reset.json());
    expect(errors, 'uncaught browser errors').toEqual([]);
  },
});

async function addLibrary(page: Page, path: string, shows = false) {
  const response = await page.request.post('/api/libraries', { data: { name: 'Recovery library', kind: shows ? 'shows' : 'movies', paths: [path] } });
  expect(response.ok()).toBeTruthy();
  const lib = await response.json();
  await expect.poll(async () => (await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json()).running).toBe(false);
  return (await (await page.request.get(`/api/items?library_id=${lib.id}&kind=${shows ? "series" : "movie"}`)).json()).items;
}

for (const shows of [false, true]) {
  test(`browse jump: real ${shows ? 'series' : 'movie'} title boundary, filters, reverse and reset`, async ({ page, media }, info) => {
    const items: Item[] = await addLibrary(page, shows ? media.shows : media.movies, shows);
    const target = items[0];
    expect((await page.request.patch(`/api/items/${target.id}`, { data: { title: 'The Sierra', sort_title: 'sierra' } })).ok()).toBeTruthy();
    const lib = (await (await page.request.get('/api/libraries')).json())[0];
    await page.goto(`/library/${lib.id}?unwatched=1`);
    const jump = page.getByRole('combobox', { name: 'Jump to letter', exact: true });
    await jump.selectOption('s');
    await expect(page).toHaveURL(/unwatched=1&letter=s/);
    await expect(page.getByRole('status')).toContainText('Starting at S');
    await expect(page.getByRole('link', { name: 'The Sierra' })).toBeVisible();
    const query = await (await page.request.get(`/api/items?library_id=${lib.id}&jump=S&unwatched=1&kind=${shows ? "series" : "movie"}`)).json();
    expect(query.total).toBe(items.length);
    expect(query.items[0].id).toBe(target.id);
    await page.getByRole('button', { name: 'Reverse sort direction' }).click();
    await expect(page.getByRole('status')).toContainText('Z–A');
    await expect(page.getByRole('link', { name: 'The Sierra' })).toBeVisible();
    await jump.selectOption('');
    await expect(page).not.toHaveURL(/letter=/);
    await expect(page.getByRole('button', { name: 'Unwatched', exact: true })).toHaveAttribute('aria-pressed', 'true');
    await jump.selectOption('s');
    await page.getByRole('button', { name: 'Title', exact: true }).click();
    await page.getByRole('menuitemradio', { name: 'Recently Added', exact: true }).click();
    await expect(jump).toHaveCount(0);
    await expect(page).not.toHaveURL(/letter=/);
    await page.screenshot({ path: info.outputPath('ordinary-sort.png'), fullPage: true });
  });
}

test('browse jump: unloaded boundary, continuation, loaded selection and Back position', async ({ page, media }, info) => {
  const originals: Item[] = await addLibrary(page, media.movies);
  const template = originals[0];
  const lib = (await (await page.request.get('/api/libraries')).json())[0];
  const entries = Array.from({ length: 1016 }, (_, i) => ({ ...template,
    id: i === 800 ? template.id : `jump-${i}`,
    title: i < 510 ? `Alpha ${String(i).padStart(4, '0')}` : i < 1015 ? `Sierra ${String(i-510).padStart(4, '0')}` : 'Zulu',
  }));
  const requests: URL[] = [];
  await page.route('**/api/items?*', route => {
    const url = new URL(route.request().url());
    if (url.searchParams.get('library_id') !== String(lib.id)) return route.continue();
    requests.push(url);
    const offset = url.searchParams.get('jump') === 's' ? 510 : Number(url.searchParams.get('offset') || 0);
    return route.fulfill({ json: { items: entries.slice(offset, offset + 500), total: entries.length, offset } });
  });
  await page.goto(`/library/${lib.id}`);
  await expect(page.getByRole('link', { name: 'Alpha 0000' })).toBeVisible();
  await page.getByRole('combobox', { name: 'Jump to letter' }).selectOption('s');
  await expect(page.getByRole('link', { name: 'Sierra 0000' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Alpha 0000' })).toHaveCount(0);
  expect(requests.some(url => url.searchParams.get('jump') === 's')).toBe(true);
  await page.screenshot({ path: info.outputPath('jump-desktop.png') });
  await page.getByRole('button', { name: 'Select', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Select loaded (500)', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Stop choosing', exact: true }).click();
  await page.getByRole('button', { name: 'Load more', exact: true }).click();
  await expect(page.getByRole('link', { name: /^Zulu\b/ })).toBeVisible();
  expect(requests.some(url => url.searchParams.get('offset') === '1010' && !url.searchParams.has('jump'))).toBe(true);
  await expect(page.getByRole('button', { name: 'Load more', exact: true })).toHaveCount(0);
  const target = page.getByRole('link', { name: /^Sierra 0290\b/ });
  await target.scrollIntoViewIfNeeded();
  const y = await page.evaluate(() => window.scrollY);
  expect(y).toBeGreaterThan(1000);
  // A title can open in the same task as a final scroll, before its scroll event.
  // Record the departure position at navigation rather than relying on that event.
  const departureY = await target.evaluate((link: HTMLAnchorElement) => {
    window.scrollTo(0, window.scrollY - 200);
    const departure = window.scrollY;
    link.click();
    return departure;
  });
  await expect(page).toHaveURL(new RegExp(`/movie/${template.id}`));
  await expect(page.getByRole('heading', { level: 1, name: template.title })).toBeVisible();
  await page.goBack();
  await expect(page.getByRole('combobox', { name: 'Jump to letter' })).toHaveValue('s');
  await expect.poll(async () => Math.abs(await page.evaluate(() => window.scrollY) - departureY)).toBeLessThan(5);
  await expect(target).toBeVisible();
  await page.screenshot({ path: info.outputPath('back-at-letter.png') });
});

test('browse jump: phone, request failure Retry and empty-direction recovery', async ({ page, media }, info) => {
  const items: Item[] = await addLibrary(page, media.movies);
  const lib = (await (await page.request.get('/api/libraries')).json())[0];
  await page.request.patch(`/api/items/${items[0].id}`, { data: { title: 'Sierra', sort_title: 'sierra' } });
  let fail = true;
  await page.route('**/api/items?*', route => {
    if (fail && new URL(route.request().url()).searchParams.get('jump') === 's') return route.fulfill({ status: 503, json: { error: 'Jump unavailable' } });
    return route.continue();
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`/library/${lib.id}?unwatched=1`);
  const jump = page.getByRole('combobox', { name: 'Jump to letter' });
  await jump.selectOption('s');
  await expect(page.getByText('Jump unavailable', { exact: true })).toBeVisible();
  fail = false;
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByRole('link', { name: /^Sierra\b/ })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: info.outputPath('jump-phone.png'), fullPage: true });
  await jump.selectOption('z');
  await expect(page.getByRole('heading', { name: 'No titles in this direction' })).toBeVisible();
  await jump.selectOption('');
  await expect(page.getByRole('link', { name: /^Sierra\b/ })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Unwatched', exact: true })).toHaveAttribute('aria-pressed', 'true');
});

