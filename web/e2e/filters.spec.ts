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
async function seed(page: Page) {
  const r = await page.request.post('/api/libraries', { data: { name: 'Formats', kind: 'movies', paths: [media.playback] } });
  expect(r.ok()).toBe(true); const lib: Library = await r.json();
  await expect.poll(async () => (await (await page.request.get('/api/libraries/' + lib.id + '/scan')).json()).running, { timeout: 30000 }).toBe(false);
  const items: Item[] = (await (await page.request.get('/api/items?library_id=' + lib.id)).json()).items;
  return { lib, items };
}
async function setup(page: Page) {
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Choose a database', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Continue', exact: true }).click();
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


// Richer filters (docs/FILTER-DISCOVERY-API.md): the library offers the video and audio formats its
// files actually have, with counts; choosing one narrows the titles, shows as a chip, lives in
// the address, and the watch status choices exclude the Unwatched switch.
test('filters: video formats from the library, watch status, and the address keeps them', async ({ page }, info) => {
  const { lib, items } = await seed(page);
  expect(items.map((i) => i.title).sort()).toEqual(['Remux Movie', 'Transcode Movie']);
  const facets = await (await page.request.get(`/api/libraries/${lib.id}/facets`)).json();
  expect(facets.video_codecs.map((f: { value: string }) => f.value).sort()).toEqual(['h264', 'mpeg2video']);

  await page.goto(`/library/${lib.id}`);
  const grid = page.getByRole('main');
  await expect(grid.getByText('Remux Movie', { exact: true }).first()).toBeVisible({ timeout: 15_000 });
  await page.getByRole('button', { name: 'Filters', exact: true }).click();
  const panel = page.getByRole('dialog', { name: 'Filters' });
  await expect(panel.getByRole('heading', { name: 'Video and audio', exact: true })).toBeVisible();
  const video = panel.getByRole('combobox', { name: 'Video format', exact: true });
  await expect(video.locator('option')).toHaveText(['Any video format', 'H.264 (1)', 'MPEG-2 (1)']);
  // Only one audio format in this library: nothing to choose, so it isn't offered.
  await expect(panel.getByRole('combobox', { name: 'Audio format', exact: true })).toHaveCount(0);
  await page.screenshot({ path: info.outputPath('filters-panel.png') });
  await video.selectOption({ label: 'MPEG-2 (1)' });
  await expect(page).toHaveURL(/video_codec=mpeg2video/);
  await page.keyboard.press('Escape');
  await expect(grid.getByText('Transcode Movie', { exact: true }).first()).toBeVisible();
  await expect(grid.getByText('Remux Movie', { exact: true })).toHaveCount(0);
  const chip = page.getByRole('button', { name: 'Remove filter: MPEG-2', exact: true });
  await expect(chip).toBeVisible();
  await page.screenshot({ path: info.outputPath('filtered.png') });

  // A reload keeps it (it's in the address); the chip turns it off.
  await page.reload();
  await expect(grid.getByText('Remux Movie', { exact: true })).toHaveCount(0);
  await chip.click();
  await expect(grid.getByText('Remux Movie', { exact: true }).first()).toBeVisible();

  // Watch status: Watched finds the watched title; the Unwatched switch replaces it.
  const remux = items.find((i) => i.title === 'Remux Movie')!;
  expect((await page.request.put(`/api/items/${remux.id}/played`, { data: { played: true } })).ok()).toBe(true);
  await page.getByRole('button', { name: 'Filters', exact: true }).click();
  await panel.getByRole('combobox', { name: 'Watch status', exact: true }).selectOption({ label: 'Watched' });
  await expect(page).toHaveURL(/watch_state=played/);
  await expect(grid.getByText('Transcode Movie', { exact: true })).toHaveCount(0);
  await expect(grid.getByText('Remux Movie', { exact: true }).first()).toBeVisible();
  await panel.getByRole('switch', { name: "Only titles I haven't watched", exact: true }).click();
  await expect(page).not.toHaveURL(/watch_state/);
  await expect(panel.getByRole('combobox', { name: 'Watch status', exact: true })).toHaveValue('');
  await expect(grid.getByText('Transcode Movie', { exact: true }).first()).toBeVisible();
  await expect(grid.getByText('Remux Movie', { exact: true })).toHaveCount(0);
});

test('filters: a custom Home row can be made of a video format', async ({ page }) => {
  const { lib } = await seed(page);
  const prefs = await (await page.request.get('/api/me/prefs')).json();
  const rows = [...(prefs.home ?? []), { id: 'custom:mpeg2', title: 'Old formats', filter: { library_id: lib.id, video_codec: 'mpeg2video' } }];
  const saved = await page.request.patch('/api/me/prefs', { data: { home: rows } });
  expect(saved.ok(), await saved.text()).toBe(true);
  await page.goto('/');
  const row = page.getByRole('region', { name: 'Old formats', exact: true });
  await expect(row.getByText('Transcode Movie', { exact: true }).first()).toBeVisible({ timeout: 15_000 });
  await expect(row.getByText('Remux Movie', { exact: true })).toHaveCount(0);
  // The row's link opens the library with the same filter.
  await row.getByRole('link', { name: 'Old formats' }).click();
  await expect(page).toHaveURL(/video_codec=mpeg2video/);
});
