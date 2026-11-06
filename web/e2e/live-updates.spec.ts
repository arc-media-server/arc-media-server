import { test, expect, type Page } from '@playwright/test';
import type { Item, Library } from '../src/lib/api';
type Fixtures = { movies: string; shows: string; empty: string; playback: string };
const password = 'Arc-browser-test-123';
let media: Fixtures;

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

// Live updates (src/lib/live-updates.ts): a change made elsewhere shows on an open page without a
// reload, through one events stream per tab.
test('live updates: a title renamed elsewhere updates the open page and Home without reloading', async ({ page }) => {
  const streams: string[] = [];
  page.on('request', (r) => { if (new URL(r.url()).pathname === '/api/v1/events') streams.push(r.url()); });
  const { item } = await seed(page);
  await page.goto('/movie/' + item.id);
  await expect(page.getByRole('heading', { name: item.title, exact: true })).toBeVisible();
  await expect.poll(() => streams.length).toBe(1);

  // As another device would: rename it through the API, not this page.
  const renamed = 'Renamed Elsewhere';
  const r = await page.request.patch('/api/items/' + item.id, { data: { title: renamed } });
  expect(r.ok()).toBe(true);
  await expect(page.getByRole('heading', { name: renamed, exact: true })).toBeVisible({ timeout: 10_000 });

  // Home, opened afterwards in the same tab, shows it too, and the tab still has one stream.
  await page.getByRole('link', { name: 'Home', exact: true }).first().click();
  await expect(page.locator('[data-item="' + item.id + '"]').filter({ visible: true }).first()).toBeVisible();
  const before = streams.length;
  const second = await page.request.patch('/api/items/' + item.id, { data: { title: 'Renamed Again' } });
  expect(second.ok()).toBe(true);
  await expect(page.getByText('Renamed Again').filter({ visible: true }).first()).toBeVisible({ timeout: 10_000 });
  expect(streams.length).toBe(before);
});
