import { test, expect } from '@playwright/test';

for (const width of [1440, 390]) test(`browsing English labels and literal metadata at ${width}px`, async ({ page }, info) => {
  const reset = await fetch('http://127.0.0.1:18200/reset', { method: 'POST' });
  expect(reset.ok).toBe(true);
  const media = await reset.json();
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.context().route('**/*', route => {
    const url = new URL(route.request().url());
    return url.hostname === '127.0.0.1' || url.protocol === 'data:' ? route.continue() : route.abort();
  });
  await page.setViewportSize({ width, height: 900 });
  await page.emulateMedia({ reducedMotion: 'reduce' });
  expect((await page.request.post('/api/auth/setup', { data: { name: 'Owner', password: 'Browsing-fixture-123', local_bypass: false, tmdb_api_key: 'fixture-not-a-secret', region: 'GB' } })).ok()).toBe(true);
  expect((await page.request.patch('/api/settings', { data: { scan_automatic: false, scan_on_start: false, scan_interval_minutes: 0, preview_mode: 'off', detection_mode: 'off', analysis_enabled: false, metadata_refresh_enabled: false, auto_backups_enabled: false, database_upkeep_enabled: false, cache_cleanup_enabled: false } })).ok()).toBe(true);
  const library = await (await page.request.post('/api/libraries', { data: { name: 'Fixture library', kind: 'movies', paths: [media.movies] } })).json();
  await expect.poll(async () => (await (await page.request.get(`/api/libraries/${library.id}/scan`)).json()).running).toBe(false);
  const items = (await (await page.request.get(`/api/items?library_id=${library.id}`)).json()).items;
  expect(items.length).toBeGreaterThan(1);
  const capture = async (name: string) => {
    await page.evaluate(() => document.fonts.ready);
    await page.waitForTimeout(450);
    await page.screenshot({ path: info.outputPath(`${name}.png`), animations: 'disabled' });
  };
  await page.goto(`/library/${library.id}`);
  await expect(page.getByRole('heading', { name: 'Fixture library', exact: true })).toBeVisible();
  await expect(page.locator('main a[href^="/movie/"]').first()).toBeVisible();
  await capture('library');
  await page.getByRole('combobox', { name: 'Jump to letter' }).selectOption('a');
  await expect(page.getByRole('status')).toHaveText('Starting at A · A–Z. All titles');
  await capture('library-letter');
  await page.goto(`/library/${library.id}?genre=NoSuchGenre`);
  await expect(page.getByRole('heading', { name: 'Nothing matches', exact: true })).toBeVisible();
  await capture('library-empty');
  await page.goto(`/library/${library.id}?view=collections`);
  await expect(page.getByRole('heading', { name: 'No collections yet', exact: true })).toBeVisible();
  await capture('collections-empty');
  const collection = await (await page.request.post('/api/collections', { data: { library_id: library.id, kind: 'manual', title: 'Acting <&> collection', item_ids: items.slice(0, 2).map((item: { id: string }) => item.id) } })).json();
  await page.goto(`/collection/${collection.id}`);
  await expect(page.getByRole('heading', { name: 'Acting <&> collection', exact: true })).toBeVisible();
  await expect(page.getByText('2 movies', { exact: true })).toBeVisible();
  await capture('collection');
  await page.getByRole('button', { name: 'Arrange', exact: true }).click();
  await expect(page.getByRole('button', { name: `Take ${items[0].title} out`, exact: true })).toBeVisible();
  await capture('collection-arrange');
  await page.getByRole('button', { name: 'Done', exact: true }).click();
  await page.getByRole('button', { name: 'Delete', exact: true }).click();
  await expect(page.getByRole('alertdialog', { name: 'Delete Acting <&> collection?', exact: true })).toBeVisible();
  await capture('collection-delete');
  await page.keyboard.press('Escape');
  const person = { id: 999, name: 'Acting <&> Person', image: '', known_for: 'actor', count: 2 };
  // Presentation fixtures deliberately cover all sections and role labels; import/matching
  // correctness is covered separately by the existing real-provider browser journeys.
  await page.route('**/api/search?*', route => route.fulfill({ json: { movies: items.slice(0, 1), shows: [], episodes: [], people: [person], collections: [{ ...collection, count: 2 }], genres: [{ library_id: library.id, library_name: library.name, name: 'Romance <&>', count: 2 }] } }));
  await page.goto('/search?q=%3Cb%3E%26');
  await expect(page.getByRole('heading', { name: 'Results for “<b>&”', exact: true })).toBeVisible();
  await expect(page.getByText('Acting · 2 titles', { exact: true })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Romance <&> Fixture library · 2', exact: true })).toBeVisible();
  await capture('search');
  await page.route('**/api/people/999', route => route.fulfill({ json: { person, credits: [ { item: items[0], roles: ['actor'], character: 'Literal <&> character' }, { item: items[1], roles: ['director', 'writer', 'creator'] } ] } }));
  await page.goto('/person/999');
  await expect(page.getByRole('heading', { name: person.name, exact: true })).toBeVisible();
  await expect(page.getByText('2 titles in your libraries', { exact: true })).toBeVisible();
  await expect(page.getByText('Literal <&> character', { exact: true })).toBeVisible();
  await expect(page.getByText('Directing, Writing, Created', { exact: true })).toBeVisible();
  await capture('person');
  expect(errors).toEqual([]);
});
