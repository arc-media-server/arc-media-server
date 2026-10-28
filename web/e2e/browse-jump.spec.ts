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

const rail = (page: Page) => page.getByRole('navigation', { name: 'Jump to a letter', exact: true });
const inView = (page: Page, name: RegExp | string) => expect(page.getByRole('link', { name })).toBeInViewport();

for (const shows of [false, true]) {
  test(`letter rail: real ${shows ? 'series' : 'movie'} titles, filters, reverse and other sorts`, async ({ page, media }, info) => {
    const items: Item[] = await addLibrary(page, shows ? media.shows : media.movies, shows);
    const target = items[0];
    expect((await page.request.patch(`/api/items/${target.id}`, { data: { title: 'The Sierra', sort_title: 'sierra' } })).ok()).toBeTruthy();
    const lib = (await (await page.request.get('/api/libraries')).json())[0];
    const firstPage = page.waitForRequest(r => r.url().includes('/api/items?') && new URL(r.url()).searchParams.get('letters') === '1');
    await page.goto(`/library/${lib.id}?unwatched=1`);
    // The letters come with the filtered list's first page.
    expect(new URL((await firstPage).url()).searchParams.get('unwatched')).toBe('true');
    await expect(rail(page).getByRole('button', { name: 'Jump to Q', exact: true })).toBeDisabled();
    await rail(page).getByRole('button', { name: 'Jump to S', exact: true }).click();
    await expect(rail(page).getByRole('button', { name: 'Jump to S', exact: true })).toHaveAttribute('aria-current', 'true');
    await inView(page, /^The Sierra\b/);
    const letters = (await (await page.request.get(`/api/items?library_id=${lib.id}&letters=1&unwatched=1&kind=${shows ? 'series' : 'movie'}`)).json()).letters;
    expect(letters.reduce((n: number, l: { count: number }) => n + l.count, 0)).toBe(items.length);
    expect(letters.some((l: { letter: string }) => l.letter === 's')).toBe(true);
    // Z–A from the sort menu keeps the filters, and the rail still finds the letter.
    await page.getByRole('button', { name: 'Order: A–Z', exact: true }).click();
    await page.getByRole('menuitemradio', { name: 'Z–A', exact: true }).click();
    await expect(page).toHaveURL(/unwatched=1.*reverse=1|reverse=1.*unwatched=1/);
    await rail(page).getByRole('button', { name: 'Jump to S', exact: true }).click();
    await inView(page, /^The Sierra\b/);
    await expect(page.getByRole('button', { name: 'Remove filter: Unwatched', exact: true })).toBeVisible();
    // Other sorts have no letters to jump to.
    await page.getByRole('button', { name: 'Sort by Title', exact: true }).click();
    await page.getByRole('menuitemradio', { name: 'Recently Added', exact: true }).click();
    await expect(rail(page)).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Order: Oldest first', exact: true })).toBeVisible();
    await page.screenshot({ path: info.outputPath('ordinary-sort.png'), fullPage: true });
  });
}

// 1016 titles: Alpha (510), Sierra (505), Zulu. The second and third pages aren't loaded until needed.
function bigLibrary(template: Item) {
  return Array.from({ length: 1016 }, (_, i) => ({ ...template,
    id: i === 800 ? template.id : `jump-${i}`,
    title: i < 510 ? `Alpha ${String(i).padStart(4, '0')}` : i < 1015 ? `Sierra ${String(i-510).padStart(4, '0')}` : 'Zulu',
  }));
}
const bigLetters = [{ letter: 'a', count: 510 }, { letter: 's', count: 505 }, { letter: 'z', count: 1 }];

test('letter rail: loads the pages up to an unloaded letter, keeps the titles before it, and Back returns', async ({ page, media }, info) => {
  const originals: Item[] = await addLibrary(page, media.movies);
  const template = originals[0];
  const lib = (await (await page.request.get('/api/libraries')).json())[0];
  const entries = bigLibrary(template);
  const requests: URL[] = [];
  await page.route('**/api/items?*', route => {
    const url = new URL(route.request().url());
    if (url.searchParams.get('library_id') !== String(lib.id)) return route.continue();
    requests.push(url);
    const offset = Number(url.searchParams.get('offset') || 0);
    return route.fulfill({ json: { items: entries.slice(offset, offset + 500), total: entries.length, offset,
      ...(url.searchParams.get('letters') === '1' ? { letters: bigLetters } : {}) } });
  });
  await page.goto(`/library/${lib.id}`);
  await expect(page.getByRole('link', { name: 'Alpha 0000' })).toBeVisible();
  await rail(page).getByRole('button', { name: 'Jump to S', exact: true }).click();
  await inView(page, /^Sierra 0000\b/);
  // The whole list stays: the titles before the letter are still there to scroll back to.
  await expect(page.getByRole('link', { name: 'Alpha 0000' })).toHaveCount(1);
  expect(requests.some(url => url.searchParams.get('offset') === '500' && !url.searchParams.has('jump'))).toBe(true);
  await page.screenshot({ path: info.outputPath('jump-desktop.png') });
  await page.getByRole('button', { name: 'More for Recovery library', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Select titles', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Select loaded (1000)', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Stop choosing', exact: true }).click();
  await rail(page).getByRole('button', { name: 'Jump to Z', exact: true }).click();
  await inView(page, /^Zulu\b/);
  expect(requests.some(url => url.searchParams.get('offset') === '1000')).toBe(true);
  await expect(page.getByRole('button', { name: 'Load more', exact: true })).toHaveCount(0);
  const target = page.getByRole('link', { name: /^Sierra 0290\b/ });
  await target.scrollIntoViewIfNeeded();
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
  await expect.poll(async () => Math.abs(await page.evaluate(() => window.scrollY) - departureY)).toBeLessThan(5);
  await expect(target).toBeVisible();
  await page.screenshot({ path: info.outputPath('back-at-letter.png') });
});

test('letter rail: phone layout, a failed page load Retry, and older ?letter= links', async ({ page, media }, info) => {
  const originals: Item[] = await addLibrary(page, media.movies);
  const lib = (await (await page.request.get('/api/libraries')).json())[0];
  const entries = bigLibrary(originals[0]);
  let fail = true;
  await page.route('**/api/items?*', route => {
    const url = new URL(route.request().url());
    if (url.searchParams.get('library_id') !== String(lib.id)) return route.continue();
    const offset = Number(url.searchParams.get('offset') || 0);
    if (fail && offset === 500) return route.fulfill({ status: 503, json: { error: 'Jump unavailable' } });
    return route.fulfill({ json: { items: entries.slice(offset, offset + 500), total: entries.length, offset,
      ...(url.searchParams.get('letters') === '1' ? { letters: bigLetters } : {}) } });
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`/library/${lib.id}`);
  await rail(page).getByRole('button', { name: 'Jump to S', exact: true }).click();
  await expect(page.getByText('Jump unavailable', { exact: true }).first()).toBeVisible();
  fail = false;
  await page.getByRole('button', { name: 'Retry', exact: true }).first().click();
  await inView(page, /^Sierra 0000\b/);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: info.outputPath('jump-phone.png') });
  // A link from before the rail (?letter=s) jumps there and drops the letter from the address.
  await page.goto(`/library/${lib.id}?letter=s`);
  await inView(page, /^Sierra 0000\b/);
  await expect(page).not.toHaveURL(/letter=/);
});
