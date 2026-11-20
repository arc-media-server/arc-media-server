import { test, expect } from '@playwright/test';
import { mkdir, copyFile, rm } from 'node:fs/promises';
import { join } from 'node:path';

test('Recently Added: real initial import makes a season card and one episode makes an episode card', async ({ page }) => {
  const reset = await fetch('http://127.0.0.1:18200/reset', { method: 'POST' });
  expect(reset.ok).toBeTruthy();
  const media = await reset.json();
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.context().route('**/*', route => {
    const u = new URL(route.request().url());
    return u.hostname === '127.0.0.1' || u.protocol === 'data:' ? route.continue() : route.abort();
  });
  expect((await page.request.post('/api/auth/setup', { data: { name: 'Batches owner', password: 'Batches-fixture-password', local_bypass: false, tmdb_api_key: 'fixture-key-not-a-secret' } })).ok()).toBeTruthy();
  expect((await page.request.patch('/api/settings', { data: { scan_automatic: false, scan_on_start: false, scan_interval_minutes: 0, preview_mode: 'off', detection_mode: 'off', analysis_enabled: false, metadata_refresh_enabled: false, auto_backups_enabled: false } })).ok()).toBeTruthy();
  // Only the two provider-known episodes; the full shared fixture also contains an unplaced file.
  const folder = join(media.empty, 'Arc Show (2020)', 'Season 01');
  await mkdir(folder, { recursive: true });
  for (const n of [1, 2]) await copyFile(join(media.shows, 'Arc Show (2020)', 'Season 01', `Arc Show - S01E0${n}.mp4`), join(folder, `Arc Show - S01E0${n}.mp4`));
  try {
    const created = await page.request.post('/api/libraries', { data: { name: 'Batch imports', kind: 'shows', source_ordering: 'tmdb_aired', paths: [media.empty] } });
    expect(created.ok()).toBeTruthy();
    const lib = await created.json();
    await expect.poll(async () => (await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json()).running).toBe(false);
    const cards = await (await page.request.get(`/api/libraries/${lib.id}/recent-shows`)).json();
    expect(cards).toHaveLength(1);
    expect(cards[0]).toMatchObject({ type: 'season', season_number: 1, episodes: 2 });
    await page.goto('/');
    const row = page.getByRole('region', { name: 'Recently Added in Batch imports', exact: true });
    await expect(row.getByText('Season 1', { exact: true })).toBeVisible();
    await expect(row.locator(`a[href="/series/${cards[0].item.id}"]`)).toHaveCount(1);
    await rm(join(folder, 'Arc Show - S01E02.mp4'));
    expect((await page.request.post(`/api/libraries/${lib.id}/scan`)).status()).toBe(202);
    await expect.poll(async () => (await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json()).running).toBe(false);
    const single = await (await page.request.get(`/api/libraries/${lib.id}/recent-shows`)).json();
    expect(single).toHaveLength(1);
    expect(single[0]).toMatchObject({ type: 'episode', episodes: 1 });
    expect(single[0].item.kind).toBe('episode');
    await page.reload();
    await expect(row.getByText('Season 1', { exact: true })).toHaveCount(0);
    const episodeCard = row.locator(`[data-item="${single[0].item.id}"]`);
    await expect(episodeCard).toHaveCount(1);
    await expect(episodeCard.locator('a > div').last().locator('p')).toHaveText([single[0].item.series_title, single[0].item.title, 'S1 · E1']);
    expect(errors).toEqual([]);
  } finally {
    await rm(join(media.empty, 'Arc Show (2020)'), { recursive: true, force: true });
  }
});

test('Recently Added: repeated batches keep counts, artwork and navigation when the row updates', async ({ page }, info) => {
  const reset = await fetch('http://127.0.0.1:18200/reset', { method: 'POST' });
  expect(reset.ok).toBeTruthy();
  const media = await reset.json();
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.context().route('**/*', route => {
    const u = new URL(route.request().url());
    return u.hostname === '127.0.0.1' || u.protocol === 'data:' ? route.continue() : route.abort();
  });
  expect((await page.request.post('/api/auth/setup', { data: { name: 'Batches owner', password: 'Batches-fixture-password', local_bypass: false, tmdb_api_key: 'fixture-key-not-a-secret' } })).ok()).toBeTruthy();
  expect((await page.request.patch('/api/settings', { data: { scan_automatic: false, scan_on_start: false, scan_interval_minutes: 0, preview_mode: 'off', detection_mode: 'off', analysis_enabled: false, metadata_refresh_enabled: false, auto_backups_enabled: false } })).ok()).toBeTruthy();
  const created = await page.request.post('/api/libraries', { data: { name: 'Batch layout', kind: 'shows', paths: [media.shows] } });
  expect(created.ok()).toBeTruthy();
  const lib = await created.json();
  await expect.poll(async () => (await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json()).running).toBe(false);
  const show = (await (await page.request.get(`/api/items?library_id=${lib.id}&kind=series`)).json()).items[0];
  // Explicit presentation fixtures; the actual grouping/clock/access rules have separate Go tests.
  const picture = (color: string) => `data:image/svg+xml,${encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" width="200" height="300"><rect width="200" height="300" fill="${color}"/></svg>`)}`;
  const seriesArt = picture('navy'), seasonArt = picture('teal');
  const item = { ...show, images: { ...show.images, primary: seriesArt } };
  let batches = [
    { type: 'season', item, season_number: 1, episodes: 2, added_at: 200000, season_poster: seasonArt },
    { type: 'season', item, season_number: 1, episodes: 3, added_at: 100000, season_poster: undefined },
    { type: 'series', item, episodes: 6, added_at: 10000 },
  ];
  await page.route(`**/api/libraries/${lib.id}/recent-shows`, route => route.fulfill({ json: batches }));
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto('/');
    const row = page.getByRole('region', { name: 'Recently Added in Batch layout', exact: true });
    await expect(row.getByText('Season 1', { exact: true })).toHaveCount(2);
    await expect(row.getByText(/Season 1 ·/)).toHaveCount(0);
    // The new-episode count is the season card's corner badge.
    await expect(row.getByLabel('2 new episodes', { exact: true })).toHaveCount(1);
    await expect(row.getByLabel('3 new episodes', { exact: true })).toHaveCount(1);
    await expect(row.getByText('6 new episodes', { exact: true })).toBeAttached();
    await expect(row.locator(`a[href="/series/${show.id}"]`)).toHaveCount(3);
    const rendered = row.locator(`[data-item="${show.id}"]`);
    await expect(rendered.nth(0).locator('img')).toHaveAttribute('src', seasonArt);
    await expect(rendered.nth(1).locator('img')).toHaveAttribute('src', seriesArt); // missing season artwork falls back
    await expect(rendered.nth(2).locator('img')).toHaveAttribute('src', seriesArt);
    await row.scrollIntoViewIfNeeded();
    await expect.poll(() => rendered.nth(0).locator('img').evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth > 0)).toBe(true);
    await page.screenshot({ path: info.outputPath(`batch-cards-${width}.png`) });
  }
  // A real metadata invalidation refetches the query while Home stays mounted.
  batches = [
    { type: 'season', item, season_number: 1, episodes: 4, added_at: 300000, season_poster: seasonArt },
    { type: 'season', item, season_number: 1, episodes: 2, added_at: 200000, season_poster: seasonArt },
  ];
  expect((await page.request.patch(`/api/items/${show.id}`, { data: { title: 'Batch refresh trigger' } })).ok()).toBeTruthy();
  const row = page.getByRole('region', { name: 'Recently Added in Batch layout', exact: true });
  await expect(row.getByText('Season 1', { exact: true })).toHaveCount(2);
  await expect(row.locator(`[data-item="${show.id}"] img`)).toHaveCount(2);
  for (const img of await row.locator(`[data-item="${show.id}"] img`).all()) await expect(img).toHaveAttribute('src', seasonArt);
  await expect(row.getByText('6 new episodes', { exact: true })).toHaveCount(0);
  await expect(row.locator(`a[href="/series/${show.id}"]`)).toHaveCount(2);
  expect(errors).toEqual([]);
});
