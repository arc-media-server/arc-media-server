import { test, expect } from '@playwright/test';

for (const locale of ['en-US', 'fr-FR']) {
  test.describe(`bundled English with ${locale} browser`, () => {
    test.use({ locale });
    test('first paint and profile interpolation retain English without nonexistent locale requests', async ({ page }) => {
      expect((await fetch('http://127.0.0.1:18200/reset', { method: 'POST' })).ok).toBe(true);
      const errors: string[] = [];
      const localeRequests: string[] = [];
      page.on('pageerror', error => errors.push(error.message));
      page.on('request', request => { if (/locales|\/fr[/-]|\/en[/-]|auth-[\w-]+\.js|common-[\w-]+\.js/.test(new URL(request.url()).pathname)) localeRequests.push(request.url()); });
      await page.context().route('**/*', route => new URL(route.request().url()).hostname === '127.0.0.1' ? route.continue() : route.abort());
      await page.addInitScript(() => {
        const headings: string[] = [];
        Object.assign(window, { firstHeadings: headings });
        new MutationObserver(() => {
          const text = document.querySelector('h1')?.textContent;
          if (text && headings.at(-1) !== text) headings.push(text);
        }).observe(document, { childList: true, subtree: true, characterData: true });
      });
      await page.goto('/');
      await expect(page.getByRole('heading', { name: 'Welcome', exact: true })).toBeVisible();
      expect(await page.evaluate(() => (window as Window & { firstHeadings?: string[] }).firstHeadings?.[0])).toBe('Welcome');
      const name = 'Owner <b>& "Name"';
      expect((await page.request.post('/api/auth/setup', { data: { name, password: 'Arc-browser-test-123', local_bypass: false } })).ok()).toBe(true);
      await page.reload();
      await expect(page.getByRole('heading', { name: 'Welcome to Arc', exact: true })).toBeVisible();
      await page.setViewportSize({ width: 390, height: 900 });
      await expect(page.getByRole('button', { name: `Profile: ${name}`, exact: true })).toBeVisible();
      await expect(page.locator('b')).toHaveCount(0);
      expect(localeRequests).toEqual([]);
      expect(errors).toEqual([]);
    });
  });
}

test('recent season captions omit counts and episode captions use three lines on desktop and phone', async ({ page }, info) => {
  const reset = await fetch('http://127.0.0.1:18200/reset', { method: 'POST' });
  expect(reset.ok).toBe(true);
  const media = await reset.json();
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.context().route('**/*', route => {
    const url = new URL(route.request().url());
    return url.hostname === '127.0.0.1' || url.protocol === 'data:' ? route.continue() : route.abort();
  });
  expect((await page.request.post('/api/auth/setup', { data: { name: 'Owner', password: 'Episode-labels-123', local_bypass: false, tmdb_api_key: 'fixture-not-a-secret', region: 'GB' } })).ok()).toBe(true);
  expect((await page.request.patch('/api/settings', { data: { scan_automatic: false, scan_on_start: false, scan_interval_minutes: 0, preview_mode: 'off', detection_mode: 'off', analysis_enabled: false, metadata_refresh_enabled: false, auto_backups_enabled: false, database_upkeep_enabled: false, cache_cleanup_enabled: false } })).ok()).toBe(true);
  const created = await page.request.post('/api/libraries', { data: { name: 'Fixture series', kind: 'shows', paths: [media.shows] } });
  expect(created.ok()).toBe(true);
  const library = await created.json();
  await expect.poll(async () => (await (await page.request.get(`/api/libraries/${library.id}/scan`)).json()).running).toBe(false);
  const source = (await (await page.request.get(`/api/items?library_id=${library.id}&kind=series`)).json()).items[0];
  expect(source).toBeTruthy();
  // Presentation fixtures use an actual imported show's identity; grouping is tested separately.
  const episode = { ...source, id: 'caption-episode', kind: 'episode', series_id: source.id, series_title: source.title, title: 'First steps', season_number: 1, episode_number: 1 };
  const special = { ...episode, id: 'caption-special', title: 'A very long special episode title that needs to fit beneath a small poster', season_number: 0, episode_number: 3 };
  const unplaced = { ...episode, id: 'caption-unplaced', title: 'Unplaced episode fixture', season_number: undefined, episode_number: undefined };
  await page.route(`**/api/libraries/${library.id}/recent-shows`, route => route.fulfill({ json: [
    { type: 'season', item: source, season_number: 2, episodes: 8 },
    { type: 'season', item: source, season_number: 0, episodes: 2 },
    { type: 'episode', item: episode, episodes: 1 },
    { type: 'episode', item: special, episodes: 1 },
    { type: 'episode', item: unplaced, episodes: 1 },
  ] }));
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto('/');
    const row = page.getByRole('region', { name: 'Recently Added in Fixture series', exact: true });
    await expect(row.getByText('Season 2', { exact: true })).toBeAttached();
    await expect(row.getByText('Specials', { exact: true })).toBeAttached();
    await expect(row.getByText(/new episodes?/)).toHaveCount(0);
    for (const [item, code] of [[episode, 'S01 · E01'], [special, 'S00 · E03'], [unplaced, 'Unplaced episode']] as const) {
      const card = row.locator(`[data-item="${item.id}"]`);
      const lines = card.locator('a > div').last().locator('p');
      await expect(lines).toHaveText([source.title, item.title, code]);
      const boxes = await lines.evaluateAll(nodes => nodes.map(node => { const r = node.getBoundingClientRect(); return { y: r.y, height: r.height }; }));
      expect(boxes[0].y + boxes[0].height).toBeLessThanOrEqual(boxes[1].y);
      expect(boxes[1].y + boxes[1].height).toBeLessThanOrEqual(boxes[2].y);
      expect(boxes[2].height).toBeLessThanOrEqual(20);
    }
    const card = row.locator(`[data-item="${episode.id}"]`);
    await card.scrollIntoViewIfNeeded();
    await expect(card.getByText('S01 · E01', { exact: true })).toBeVisible();
    await page.screenshot({ path: info.outputPath(`recent-captions-${width}.png`) });
    await card.hover();
    await expect(card.getByRole('button', { name: `More for ${source.title}`, exact: true })).toBeVisible();
  }
  expect(errors).toEqual([]);
});
