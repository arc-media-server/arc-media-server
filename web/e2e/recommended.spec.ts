import { test as base, expect, type Page } from '@playwright/test';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

type Media = { movies: string; shows: string };
type Item = { id: string; title: string; kind: string; episode_number?: number };

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
    expect((await page.request.post('/api/auth/setup', { data: { name: 'Admin', password: 'Recommended-fixture-123', local_bypass: false, tmdb_api_key: 'fixture-not-a-secret', region: 'GB' } })).ok()).toBeTruthy();
    expect((await page.request.patch('/api/settings', { data: { scan_automatic: false, scan_on_start: false, scan_interval_minutes: 0, preview_mode: 'off', detection_mode: 'off', analysis_enabled: false, metadata_refresh_enabled: false, auto_backups_enabled: false, database_upkeep_enabled: false, cache_cleanup_enabled: false } })).ok()).toBeTruthy();
    await use(await reset.json());
    expect(errors).toEqual([]);
  },
});

async function library(page: Page, name: string, path: string, kind: 'movies' | 'shows') {
  const response = await page.request.post('/api/libraries', { data: { name, kind, paths: [path], source_ordering: 'tmdb_aired' } });
  expect(response.ok()).toBeTruthy();
  const lib = await response.json() as { id: number };
  await expect.poll(async () => (await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json()).running, { timeout: 60_000 }).toBe(false);
  return lib.id;
}

async function items(page: Page, libraryId: number, kind: string): Promise<Item[]> {
  return (await (await page.request.get(`/api/items?library_id=${libraryId}&kind=${kind}`)).json()).items;
}

async function rowTitles(page: Page) {
  const main = page.getByRole('main');
  await expect(main.locator('section h2').first()).toBeVisible();
  return main.locator('section h2').allInnerTexts();
}

test('Recommended: rows for a profile with some history, titled in the app, and the empty state', async ({ page, media }, info) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  const movies = await library(page, 'Movies', media.movies, 'movies');
  const arc = 'Arc Movie';
  const films = await items(page, movies, 'movie');
  const arcMovie = films.find(i => i.title === arc)!;
  const other = films.find(i => i.title !== arc)!; // Unmatched Mystery
  expect(arcMovie && other).toBeTruthy();
  // Part-way through Arc Movie; the other one watched.
  expect((await page.request.put(`/api/items/${arcMovie.id}/progress`, { data: { position_ms: 60_000, duration_ms: 120_000, state: 'stopped' } })).ok()).toBeTruthy();
  expect((await page.request.put(`/api/items/${other.id}/played`, { data: { played: true } })).ok()).toBeTruthy();

  await page.goto(`/library/${movies}?view=recommended`);
  expect(await rowTitles(page)).toEqual(['Continue Watching', 'Recently Released', 'Recently Added in Movies', 'Top Rated Movies', 'Recently Watched', 'Top Unwatched']);
  const main = page.getByRole('main');
  const continueRow = main.getByRole('region', { name: 'Continue Watching', exact: true });
  await expect(continueRow.getByText(arc).first()).toBeVisible();
  await expect(main.getByRole('region', { name: 'Recently Watched', exact: true }).getByText(other.title).first()).toBeVisible();
  // "See all" goes to the library sorted the same way.
  await expect(main.getByRole('link', { name: 'Top Unwatched' })).toHaveAttribute('href', `/library/${movies}?sort=rating&unwatched=1`);
  await expect(main.getByRole('link', { name: 'Recently Added in Movies' })).toHaveAttribute('href', `/library/${movies}?sort=added`);
  await page.screenshot({ path: info.outputPath('recommended-movies.png'), fullPage: true });

  // A TV library: the next episode carries on, and episodes watched are numbered.
  const shows = await library(page, 'TV', media.shows, 'shows');
  const episodes = await items(page, shows, 'episode');
  const first = episodes.find(e => e.title === 'First Chapter')!;
  expect(first).toBeTruthy();
  expect((await page.request.put(`/api/items/${first.id}/played`, { data: { played: true } })).ok()).toBeTruthy();
  await page.goto(`/library/${shows}?view=recommended`);
  const titles = await rowTitles(page);
  expect(titles).toContain('Continue Watching');
  expect(titles).toContain('Recently Watched Episodes');
  expect(titles).not.toContain('Start Watching'); // the only show has been started
  expect(titles).toContain('Top Rated Shows');
  await expect(main.getByRole('region', { name: 'Continue Watching', exact: true }).getByText(/S1 E2/).first()).toBeVisible();
  await expect(main.getByRole('region', { name: 'Recently Watched Episodes', exact: true }).getByText(/S1 E1 · First Chapter/).first()).toBeVisible();

  // A library with nothing in it has nothing to recommend.
  // A folder of its own: the shared Empty fixture gets a file from another journey.
  const folder = await mkdtemp(join(tmpdir(), 'arc-e2e-recommended-'));
  const empty = await library(page, 'Empty', folder, 'movies');
  await page.goto(`/library/${empty}?view=recommended`);
  await expect(page.getByText('Nothing to recommend yet')).toBeVisible();
  await expect(page.getByText('Recommendations appear here once this library has titles you can watch.')).toBeVisible();
  await rm(folder, { recursive: true, force: true });
});

test('Recommended: every kind of row is titled and links to more', async ({ page, media }, info) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  const movies = await library(page, 'Movies', media.movies, 'movies');
  const films = await items(page, movies, 'movie');
  // The fixtures have no genres or cast, so the rows that need them are given here.
  const row = (type: string, extra: object = {}) => ({ type, items: films, ...extra });
  await page.route(`**/api/libraries/${movies}/recommended`, route => route.fulfill({ json: { rows: [
    row('continue'), row('recently_released'), row('recently_added', { name: 'Movies' }), row('because_you_watched', { name: 'Alien' }),
    row('top_rated'), row('recently_watched'), row('rediscover'), row('more_in_genre', { name: 'Horror', genre: 'Horror' }),
    row('top_in_genre', { name: 'Science Fiction', genre: 'Science Fiction' }), row('top_by_person', { name: 'Ridley Scott', person_id: 7, role: 'director' }),
    row('top_by_person', { name: 'Sigourney Weaver', person_id: 8, role: 'actor' }), row('top_unwatched'),
  ] } }));
  await page.goto(`/library/${movies}?view=recommended`);
  expect(await rowTitles(page)).toEqual(['Continue Watching', 'Recently Released', 'Recently Added in Movies', 'Because you watched Alien',
    'Top Rated Movies', 'Recently Watched', 'Rediscover', 'More in Horror', 'Top Movies in Science Fiction', 'Top Movies by Ridley Scott',
    'Top Movies with Sigourney Weaver', 'Top Unwatched']);
  const main = page.getByRole('main');
  await expect(main.getByRole('link', { name: 'Top Movies by Ridley Scott' })).toHaveAttribute('href', '/person/7');
  await expect(main.getByRole('link', { name: 'Top Movies in Science Fiction' })).toHaveAttribute('href', `/library/${movies}?genre=Science%20Fiction&sort=rating`);
  await expect(main.getByRole('link', { name: 'More in Horror' })).toHaveAttribute('href', `/library/${movies}?genre=Horror&unwatched=1`);
  await expect(main.getByRole('link', { name: 'Recently Released' })).toHaveAttribute('href', `/library/${movies}?sort=release`);
  await expect(main.getByRole('link', { name: 'Top Rated Movies' })).toHaveAttribute('href', `/library/${movies}?sort=rating`);
  // Rows without a library view of their own aren't links.
  await expect(main.getByRole('link', { name: 'Rediscover' })).toHaveCount(0);
  await page.screenshot({ path: info.outputPath('recommended-all-rows.png'), fullPage: true });
  await main.getByRole('link', { name: 'Top Rated Movies' }).click();
  await expect(page).toHaveURL(new RegExp(`/library/${movies}\\?sort=rating$`));
});
