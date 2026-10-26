import { test as base, expect, type Page, type Locator } from '@playwright/test';

type Media = { movies: string };
const test = base.extend<{ media: Media }>({
  media: async ({ page }, use) => {
    const reset = await fetch('http://127.0.0.1:18200/reset', { method: 'POST' });
    expect(reset.ok).toBe(true);
    const errors: string[] = [];
    page.on('pageerror', e => errors.push(e.message));
    await page.context().route('**/*', route => {
      const url = new URL(route.request().url());
      return url.hostname === '127.0.0.1' || url.protocol === 'data:' ? route.continue() : route.abort();
    });
    expect((await page.request.post('/api/auth/setup', { data: { name: 'Admin', password: 'Navigation-fixture-123', local_bypass: false, tmdb_api_key: 'fixture-not-a-secret', region: 'GB' } })).ok()).toBeTruthy();
    expect((await page.request.patch('/api/settings', { data: { scan_automatic: false, scan_on_start: false, scan_interval_minutes: 0, preview_mode: 'off', detection_mode: 'off', analysis_enabled: false, metadata_refresh_enabled: false, auto_backups_enabled: false, database_upkeep_enabled: false, cache_cleanup_enabled: false } })).ok()).toBeTruthy();
    await use(await reset.json());
    expect(errors).toEqual([]);
  },
});

async function rows(page: Page, path: string, count: () => number = () => 20, hero = false) {
  const response = await page.request.post('/api/libraries', { data: { name: 'Navigation library', kind: 'movies', paths: [path] } });
  expect(response.ok()).toBeTruthy();
  const lib = await response.json();
  await expect.poll(async () => (await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json()).running).toBe(false);
  const source = (await (await page.request.get(`/api/items?library_id=${lib.id}`)).json()).items[0];
  await page.route('**/api/items?*', async route => {
    const url = new URL(route.request().url());
    if (url.searchParams.get('library_id') !== String(lib.id)) return route.continue();
    const n = Number(url.searchParams.get('limit')) === 12 ? 1 : count();
    const items = Array.from({ length: n }, (_, i) => ({ ...source, id: i === n - 1 ? source.id : `navigation-${i}`, title: `Row Movie ${i + 1}`, images: hero ? { ...source.images, backdrop: 'data:image/svg+xml;base64,' + Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" width="1600" height="900"><rect width="1600" height="900" fill="#243950"/></svg>').toString('base64') } : source.images }));
    return route.fulfill({ json: { items, total: n, offset: 0, limit: n } });
  });
  return source;
}
const title = 'Recently Added in Navigation library';
function shelf(page: Page) { return page.getByRole('region', { name: title, exact: true }); }
async function expectInside(card: Locator, frame: Locator) {
  const c = await card.boundingBox(); const f = await frame.boundingBox();
  expect(c).not.toBeNull(); expect(f).not.toBeNull();
  expect(c!.x).toBeGreaterThanOrEqual(f!.x - 2);
  expect(c!.x + c!.width).toBeLessThanOrEqual(f!.x + f!.width + 2);
}

test('row navigation: visible mouse controls reach beyond the first screen and respect both ends', async ({ page, media }, info) => {
  const source = await rows(page, media.movies);
  expect((await page.request.patch('/api/me/prefs', { data: { theme: 'dark' } })).ok()).toBeTruthy();
  await page.setViewportSize({ width: 1200, height: 700 });
  await page.goto('/');
  const row = shelf(page); const frame = row.getByRole('group', { name: `${title} cards`, exact: true });
  const left = row.getByRole('button', { name: `Scroll ${title} left`, exact: true });
  const right = row.getByRole('button', { name: `Scroll ${title} right`, exact: true });
  await page.mouse.move(1, 1);
  await expect(right).toBeVisible(); await expect(right).toHaveCSS('opacity', '1'); await expect(left).toBeDisabled();
  await right.click();
  await expect.poll(() => frame.evaluate(el => el.scrollLeft)).toBeGreaterThan(200);
  await expect(left).toBeEnabled();
  for (let i = 0; i < 8 && await right.isEnabled(); i++) {
    const target = await frame.evaluate(el => Math.min(el.scrollWidth - el.clientWidth - 2, el.scrollLeft + el.clientWidth * 0.5));
    await right.click();
    await expect.poll(async () => !(await right.isEnabled()) || (await frame.evaluate(el => el.scrollLeft)) >= target).toBe(true);
  }
  await expect(right).toBeDisabled();
  const last = row.getByRole('link', { name: /^Row Movie 20(?: |$)/ });
  await expectInside(last, frame);
  await page.screenshot({ path: info.outputPath('desktop-row-end.png') });
  await last.click();
  await expect(page).toHaveURL(new RegExp(`/movie/${source.id}$`));
});

test('row navigation: keyboard, focused cards, native horizontal wheel and narrow widths', async ({ page, media }, info) => {
  await rows(page, media.movies);
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.setViewportSize({ width: 1050, height: 700 });
  await page.goto('/');
  const row = shelf(page); const frame = row.getByRole('group', { name: `${title} cards`, exact: true });
  await frame.focus(); await frame.press('ArrowRight');
  await expect.poll(() => frame.evaluate(el => el.scrollLeft)).toBeGreaterThan(200);
  await frame.press('End');
  await expect(row.getByRole('button', { name: `Scroll ${title} right`, exact: true })).toBeDisabled();
  await frame.press('Home');
  await expect.poll(() => frame.evaluate(el => el.scrollLeft)).toBeLessThan(2);
  await row.getByRole('link', { name: /^Row Movie 20(?: |$)/ }).focus();
  await expectInside(row.getByRole('link', { name: /^Row Movie 20(?: |$)/ }), frame);
  await frame.focus(); await frame.press('Home');
  await frame.hover(); await page.mouse.wheel(450, 0);
  await expect.poll(() => frame.evaluate(el => el.scrollLeft)).toBeGreaterThan(100);
  await page.setViewportSize({ width: 390, height: 800 });
  await frame.focus(); await frame.press('Home');
  const next = row.getByRole('button', { name: `Scroll ${title} right`, exact: true });
  await expect(next).toBeVisible(); await next.click();
  await expect.poll(() => frame.evaluate(el => el.scrollLeft)).toBeGreaterThan(100);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: info.outputPath('phone-row-navigation.png') });
});

test('row navigation: controls follow actual overflow after resize and data updates', async ({ page, media }) => {
  let count = 2;
  await rows(page, media.movies, () => count);
  await page.setViewportSize({ width: 1200, height: 700 }); await page.goto('/');
  const row = shelf(page);
  await expect(row.getByRole('link', { name: /^Row Movie 2(?: |$)/ })).toBeVisible();
  await expect(row.getByRole('button', { name: /^Scroll / })).toHaveCount(0);
  await page.setViewportSize({ width: 300, height: 700 });
  await expect(row.getByRole('button', { name: `Scroll ${title} right`, exact: true })).toBeEnabled();
  await page.setViewportSize({ width: 1200, height: 700 });
  await expect(row.getByRole('button', { name: /^Scroll / })).toHaveCount(0);
  count = 20; await page.reload();
  await expect(row.getByRole('button', { name: `Scroll ${title} right`, exact: true })).toBeEnabled();
});

test('row navigation: small overflowing cast lists and extras use the same controls', async ({ page, media }, info) => {
  const source = await rows(page, media.movies);
  await page.route(`**/api/items/${source.id}`, async route => {
    const response = await route.fetch(); const item = await response.json();
    item.credits = Array.from({ length: 7 }, (_, i) => ({ role: 'actor', person_id: 9000 + i, name: `Cast Member ${i + 1}`, character: 'Fixture' }));
    item.extras = Array.from({ length: 8 }, (_, i) => ({ id: i + 1, title: `Extra ${i + 1}`, kind: 'other', stream_url: '/unused-fixture-stream', duration_ms: 1000 }));
    await route.fulfill({ response, json: item });
  });
  await page.setViewportSize({ width: 900, height: 800 }); await page.goto(`/movie/${source.id}`);
  for (const name of ['Cast', 'Extras']) {
    const row = page.getByRole('region', { name, exact: true });
    const frame = row.getByRole('group', { name: `${name} cards`, exact: true });
    await row.getByRole('button', { name: `Scroll ${name} right`, exact: true }).click();
    await expect.poll(() => frame.evaluate(el => el.scrollLeft)).toBeGreaterThan(30);
  }
  await page.screenshot({ path: info.outputPath('cast-extras-navigation.png') });
});

test('home hero: profile switch defaults On, handles failed saves and persists hiding across restart', async ({ page, media }, info) => {
  await rows(page, media.movies, () => 20, true);
  await page.goto('/');
  await expect(page.getByRole('region', { name: 'Featured titles', exact: true })).toBeVisible();
  await page.goto('/settings?section=profile');
  const toggle = page.getByRole('switch', { name: 'Show home hero', exact: true });
  await expect(toggle).toBeChecked();
  let fail = true;
  await page.route('**/api/me/prefs', route => route.request().method() === 'PATCH' && fail ? route.fulfill({ status: 503, json: { error: 'Hero preference unavailable' } }) : route.continue());
  await toggle.click();
  await expect(toggle).toBeChecked();
  const error = page.getByRole('alert').filter({ hasText: 'Hero preference unavailable' });
  await expect(error).toBeVisible();
  fail = false; await error.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(toggle).not.toBeChecked();
  expect((await (await page.request.get('/api/me/prefs')).json()).show_home_hero).toBe(false);
  await toggle.scrollIntoViewIfNeeded();
  await page.screenshot({ path: info.outputPath('profile-home-hero-switch.png') });
  let hiddenFeaturedRequests = 0;
  page.on('request', request => { const url = new URL(request.url()); if (url.pathname === '/api/items' && url.searchParams.get('limit') === '12') hiddenFeaturedRequests++; });
  await page.goto('/');
  await expect(page.getByRole('region', { name: 'Featured titles', exact: true })).toHaveCount(0);
  await expect(shelf(page)).toBeVisible();
  const box = await shelf(page).boundingBox(); expect(box!.y).toBeLessThan(220);
  await page.screenshot({ path: info.outputPath('home-hero-hidden-desktop.png') });
  expect((await fetch('http://127.0.0.1:18200/restart', { method: 'POST' })).ok).toBe(true);
  await page.reload(); await expect(shelf(page)).toBeVisible();
  await expect(page.getByRole('region', { name: 'Featured titles', exact: true })).toHaveCount(0);
  expect(hiddenFeaturedRequests).toBe(0);
  await page.goto('/settings?section=profile'); await expect(toggle).not.toBeChecked(); await toggle.click();
  await expect(toggle).toBeChecked(); await page.goto('/');
  await expect(page.getByRole('region', { name: 'Featured titles', exact: true })).toBeVisible();
});


test('row navigation: a normal mouse wheel still scrolls the page vertically', async ({ page, media }) => {
  await rows(page, media.movies);
  await page.setViewportSize({ width: 1050, height: 350 });
  await page.goto('/');
  const frame = shelf(page).getByRole('group', { name: `${title} cards`, exact: true });
  await expect(frame).toBeVisible();
  const box = await frame.boundingBox();
  await page.mouse.move(box!.x + 80, Math.min(280, box!.y + 40));
  const priorY = await page.evaluate(() => scrollY);
  const priorX = await frame.evaluate(el => el.scrollLeft);
  await page.mouse.wheel(0, 220);
  await expect.poll(() => page.evaluate(() => scrollY)).toBeGreaterThan(priorY);
  expect(await frame.evaluate(el => el.scrollLeft)).toBe(priorX);
});
