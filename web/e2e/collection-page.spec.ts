import { test as base, expect } from '@playwright/test';

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
    expect((await page.request.post('/api/auth/setup', { data: { name: 'Collection owner', password: 'Collection-page-fixture-123', local_bypass: false, tmdb_api_key: 'fixture-not-a-secret' } })).ok()).toBeTruthy();
    expect((await page.request.patch('/api/settings', { data: { scan_automatic: false, scan_on_start: false, scan_interval_minutes: 0, preview_mode: 'off', detection_mode: 'off', analysis_enabled: false, metadata_refresh_enabled: false, auto_backups_enabled: false } })).ok()).toBeTruthy();
    expect((await page.request.patch('/api/me/prefs', { data: { show_home_hero: false } })).ok()).toBeTruthy();
    const created = await page.request.post('/api/libraries', { data: { name: 'Collection library', kind: 'movies', paths: [media.movies] } });
    expect(created.ok()).toBeTruthy();
    const library = await created.json();
    await expect.poll(async () => (await (await page.request.get(`/api/libraries/${library.id}/scan`)).json()).running).toBe(false);
    await use({ ...library, playback: media.playback, empty: media.empty });
    expect(errors).toEqual([]);
  },
});

test('collection page: one Play button, then round Shuffle and More; Edit, Arrange and Delete live in More', async ({ page, library }, info) => {
  const items = (await (await page.request.get(`/api/items?library_id=${library.id}&kind=movie`)).json()).items;
  expect(items.length).toBeGreaterThan(1);
  const made = await page.request.post('/api/collections', { data: { library_id: library.id, kind: 'manual', title: 'Fixture collection', item_ids: items.slice(0, 2).map((i: { id: string }) => i.id) } });
  expect(made.ok()).toBeTruthy();
  const collection = await made.json();
  await page.emulateMedia({ colorScheme: 'dark' });
  await page.setViewportSize({ width: 1280, height: 800 });
  await page.goto(`/collection/${collection.id}`);
  await expect(page.getByRole('heading', { name: 'Fixture collection', level: 1 })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Play', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Shuffle', exact: true })).toBeVisible();
  for (const gone of ['Edit', 'Arrange', 'Delete']) await expect(page.getByRole('button', { name: gone, exact: true })).toHaveCount(0);
  await page.screenshot({ path: info.outputPath('collection-buttons.png') });
  await page.getByRole('button', { name: 'More', exact: true }).click();
  for (const name of ['Edit', 'Arrange', 'Delete']) await expect(page.getByRole('menuitem', { name, exact: true })).toBeVisible();
  await page.screenshot({ path: info.outputPath('collection-more-menu.png') });
  await page.getByRole('menuitem', { name: 'Arrange', exact: true }).click();
  const done = page.getByRole('button', { name: 'Done', exact: true });
  await expect(done).toBeVisible();
  await page.screenshot({ path: info.outputPath('collection-arranging.png') });
  await done.click();
  await expect(done).toHaveCount(0);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({ path: info.outputPath('collection-phone.png') });
});
