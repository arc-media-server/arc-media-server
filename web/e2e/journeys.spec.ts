import { test as base, expect, type Page } from '@playwright/test';

async function chooseDisplay(page: Page, label: RegExp) {
 const select=page.getByRole('combobox',{name:'Display order',exact:true});
 const value=await select.locator('option').filter({hasText:label}).first().getAttribute('value');
 if(!value)throw new Error('Display option missing');
 await select.selectOption(value);
}

/** Episode ordering is the server owner's, in Edit, Metadata & ordering. */
async function openOrdering(page: Page) {
 await page.getByRole('button',{name:'More',exact:true}).click();
 await page.getByRole('menuitem',{name:'Edit…',exact:true}).click();
 await page.getByRole('tab',{name:'Metadata & ordering',exact:true}).click();
}

type Fixtures = { movies: string; shows: string; empty: string; playback: string; audio: string; budget: string };
const password = 'Arc-browser-test-123';

const test = base.extend<{ media: Fixtures }>({
  media: async ({ page }, use) => {
    const reset = await fetch('http://127.0.0.1:18200/reset', { method: 'POST' });
    if (!reset.ok) throw new Error(await reset.text());
    const errors: string[] = [];
    page.on('pageerror', e => errors.push(e.message));
    // Tests need no internet access; third-party scripts/images must not hide dependencies.
    await page.context().route('**/*', route => {
      const u = new URL(route.request().url());
      return u.hostname === '127.0.0.1' || u.protocol === 'data:' ? route.continue() : route.abort();
    });
    await use(await reset.json());
    expect(errors, 'uncaught browser errors').toEqual([]);
  },
});

for (const shows of [false, true]) {
  test(`adding ${shows ? 'TV' : 'movies'} starts background import without Preview`, async ({ page, media }, info) => {
    await setup(page);
    let previews = 0;
    page.on('request', r => { if (r.method() === 'POST' && r.url().endsWith('/api/library-previews')) previews++; });
    const dialog = await libraryForm(page, shows ? 'Background TV' : 'Background movies', shows ? media.shows : media.movies, shows);
    await expect(dialog.getByRole('button', { name: 'Preview folders', exact: true })).not.toBeVisible();
    if (shows) await page.setViewportSize({ width: 390, height: 900 });
    await page.screenshot({ path: info.outputPath('add-library.png') });
    if (shows) await page.setViewportSize({ width: 1280, height: 720 });
    expect((await fetch('http://127.0.0.1:18200/metadata-delay?ms=1500', { method: 'POST' })).ok).toBe(true);
    try {
      const created = page.waitForResponse(r => r.url().endsWith('/api/libraries') && r.request().method() === 'POST' && r.ok());
      await dialog.getByRole('button', { name: 'Add Library', exact: true }).click();
      const lib = await (await created).json();
      await expect(dialog).not.toBeVisible();
      const status = await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json();
      expect(status.running).toBe(true);
      await expect.poll(async () => {
        const activity = await (await page.request.get('/api/activity')).json();
        return activity.running.some((task: { current?: { subject: string; stage: string; detail?: string }[] }) =>
          task.current?.some(work => work.subject.includes(shows ? 'Arc Show' : 'Arc Movie') &&
            /Matching|Fetching/.test(work.stage) && work.detail?.includes('TMDB')));
      }).toBe(true);
      await expect(page.locator('aside').getByRole('link', { name: shows ? 'Background TV' : 'Background movies', exact: true })).toBeVisible();
      expect(previews).toBe(0);
      await expect.poll(async () => (await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json()).running, { timeout: 30_000 }).toBe(false);
      const result = await (await page.request.get(`/api/items?library_id=${lib.id}`)).json();
      expect(result.items.some((i: { title: string; metadata_state: string }) => i.title === (shows ? 'Arc Show' : 'Arc Movie') && i.metadata_state === 'matched')).toBe(true);
      expect(previews).toBe(0);
    } finally {
      await fetch('http://127.0.0.1:18200/metadata-delay?ms=0', { method: 'POST' });
    }
  });
}

test('unchanged series scans report checks without rereading file details', async ({ page, media }, info) => {
  await setup(page);
  const lib = await addLibrary(page, 'Rescan series', media.shows, true);
  const first = await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json();
  expect(first.files_probed).toBe(3);
  expect((await page.request.post(`/api/libraries/${lib.id}/scan`)).status()).toBe(202);
  await expect.poll(async () => (await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json()).running).toBe(false);
  const second = await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json();
  expect(second.files_seen).toBe(3);
  expect(second.files_probed).toBe(0);
  expect(second.matched).toBe(0);
  await page.goto('/dashboard');
  await expect(page.getByText('3 files checked · 0 file details read', { exact: false })).toBeVisible();
  await page.screenshot({ path: info.outputPath('unchanged-series-activity.png') });
  await page.setViewportSize({ width: 390, height: 900 });
  await expect(page.getByText('3 files checked · 0 file details read', { exact: false })).toBeVisible();
  await page.screenshot({ path: info.outputPath('unchanged-series-activity-mobile.png') });
});

test('direct library addition reports invalid folders and can be retried', async ({ page, media }) => {
  await setup(page);
  const dialog = await libraryForm(page, 'Validated library', media.empty);
  await page.route('**/api/libraries', async route => {
    if (route.request().method() !== 'POST') return route.continue();
    const body = route.request().postDataJSON();
    await route.continue({ postData: JSON.stringify({ ...body, paths: [media.empty + '/missing-fixture-folder'] }) });
  });
  await dialog.getByRole('button', { name: 'Add Library', exact: true }).click();
  await expect(dialog.getByText(/can't open/)).toBeVisible();
  expect((await (await page.request.get('/api/libraries')).json()).length).toBe(0);
  await page.unroute('**/api/libraries');
  const created = page.waitForResponse(r => r.url().endsWith('/api/libraries') && r.request().method() === 'POST' && r.ok());
  await dialog.getByRole('button', { name: 'Add Library', exact: true }).click();
  await created;
  await expect(dialog).not.toBeVisible();
});

test('review: desktop and narrow layouts open the main everyday pages', async ({ page, media }, info) => {
  await setup(page);
  const lib = await addLibrary(page, 'Review movies', media.movies);
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    for (const path of ['/', `/library/${lib.id}`, '/search?q=Arc', '/watchlist', '/playlists', '/libraries', '/health', '/dashboard', '/insights', '/year', '/settings', '/profiles', '/backups', '/link']) {
      await page.goto(path);
      await expect(page.locator('main').getByRole('heading').first()).toBeVisible();
      await expect(page.getByText('Something went wrong', { exact: true })).toHaveCount(0);
    }
    await page.screenshot({ path: info.outputPath(`review-${width}.png`), fullPage: true });
  }
});

test('Library Health lists which titles have intro and credits markers', async ({ page, media }, info) => {
  await setup(page);
  await addLibrary(page, 'Marker shows', media.shows, true);
  await page.goto('/health');
  const section = page.getByRole('region', { name: 'Skip Intro and Credits' });
  await expect(section.getByRole('heading', { name: 'Marker shows' })).toBeVisible();
  await expect(section.getByText(/\d+ of \d+ episodes have an intro · \d+ of \d+ have credits/)).toBeVisible();
  await section.getByText('By title', { exact: true }).click();
  const show = section.getByRole('listitem').first();
  await expect(show.getByText(/Intro \d+\/\d+ · Credits \d+\/\d+/)).toBeVisible();
  await page.screenshot({ path: info.outputPath('marker-coverage.png'), fullPage: true });
  await show.getByRole('link').click();
  await expect(page).toHaveURL(/\/series\//);
});

test('review: library browse can reach title 501', async ({ page, media }, info) => {
  await setup(page);
  const lib = await addLibrary(page, 'Large library', media.movies);
  const original = await (await page.request.get(`/api/items?library_id=${lib.id}`)).json();
  const firstPage = await (await page.request.get(`/api/items?library_id=${lib.id}&limit=1&offset=0`)).json();
  const secondPage = await (await page.request.get(`/api/items?library_id=${lib.id}&limit=1&offset=1`)).json();
  expect(firstPage.total).toBe(2);
  expect(secondPage.items[0].id).not.toBe(firstPage.items[0].id);
  // Synthetic listing isolates pagination from disk/probe throughput, covered separately.
  const items = Array.from({ length: 501 }, (_, i) => ({ ...original.items[0], id: `review-${i}`, title: `Review title ${String(i).padStart(3, '0')}` }));
  await page.route('**/api/items?*', route => {
    const url = new URL(route.request().url());
    const offset = Number(url.searchParams.get('offset') ?? 0);
    const limit = Number(url.searchParams.get('limit') ?? 100);
    return route.fulfill({ json: { total: items.length, items: items.slice(offset, offset + limit) } });
  });
  await page.goto(`/library/${lib.id}`);
  await expect(page.getByText('501 movies', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Load more', exact: true }).scrollIntoViewIfNeeded();
  await page.screenshot({ path: info.outputPath('library-pagination.png') });
  await page.getByRole('button', { name: 'Load more', exact: true }).click();
  await expect(page.getByRole('link', { name: /Review title 500/ })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Load more', exact: true })).toHaveCount(0);
});

test('review: clearing Search clears the query and history restores the input', async ({ page, media }) => {
  void media;
  await setup(page);
  const search = page.locator('aside input[placeholder="Search"]');
  await search.fill('Arc');
  await expect(page).toHaveURL(/search\?q=Arc$/);
  await search.fill('');
  await expect(page).toHaveURL(/\/search$/);
  await expect(page.getByRole('heading', { name: 'Search', exact: true })).toBeVisible();
  await page.locator('aside').getByRole('link', { name: 'Watchlist', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Watchlist', exact: true })).toBeVisible();
  await search.fill('Movie');
  await expect(page).toHaveURL(/search\?q=Movie$/);
  await page.goBack();
  await expect(search).toHaveValue('');
  await page.goForward();
  await expect(search).toHaveValue('Movie');
});

test('review: Search displays a failed request instead of loading forever', async ({ page, media }, info) => {
  void media;
  await setup(page);
  await page.route('**/api/search?*', route => route.fulfill({ status: 503, json: { error: 'Review search unavailable' } }));
  await page.goto('/search?q=Arc');
  await expect(page.getByText('Review search unavailable', { exact: true })).toBeVisible();
  await page.screenshot({ path: info.outputPath('search-error.png') });
});

test('review: pending Search keeps results consistent and handles late responses and retry', async ({ page, media }, info) => {
  await setup(page);
  await addLibrary(page, 'Search fixtures', media.movies);
  await page.goto('/search?q=Arc');
  await expect(page.getByRole('link', { name: /Arc Movie/ }).first()).toBeVisible();
  let release!: () => void;
  const held = new Promise<void>(resolve => { release = resolve; });
  let seen = false;
  await page.route('**/api/search?*', async route => {
    if (new URL(route.request().url()).searchParams.get('q') !== 'NoSuchReviewTitle') return route.continue();
    seen = true;
    const response = await route.fetch();
    await held;
    await route.fulfill({ response });
  });
  try {
    await page.locator('aside input[placeholder="Search"]').fill('NoSuchReviewTitle');
    await expect.poll(() => seen).toBe(true);
    await expect(page.getByRole('heading', { name: 'Results for “NoSuchReviewTitle”' })).toBeVisible();
    await expect(page.getByRole('link', { name: /Arc Movie/ })).toHaveCount(0);
    await expect(page.getByRole('status')).toContainText('Searching');
    await expect(page.locator('[aria-busy="true"] [role="status"]')).toHaveCount(0);
    await expect(page.locator('[aria-busy="true"]')).toBeVisible();
    await page.screenshot({ path: info.outputPath('search-pending.png') });
    release();
    await expect(page.getByRole('heading', { name: 'No results', exact: true })).toBeVisible();
    await expect(page.locator('[aria-busy="true"]')).toHaveCount(0);
  } finally { release(); }
  await page.unroute('**/api/search?*');
  let releaseLate!: () => void;
  const lateHeld = new Promise<void>(resolve => { releaseLate = resolve; });
  let lateSeen = false;
  await page.route('**/api/search?*', async route => {
    if (new URL(route.request().url()).searchParams.get('q') !== 'LateOldQuery') return route.continue();
    const response = await route.fetch(); lateSeen = true;
    await lateHeld; await route.fulfill({ response });
  });
  try {
    await page.locator('aside input[placeholder="Search"]').fill('LateOldQuery');
    await expect.poll(() => lateSeen).toBe(true);
    await page.locator('aside input[placeholder="Search"]').fill('Arc');
    await expect(page.getByRole('heading', { name: 'Results for “Arc”' })).toBeVisible();
    await expect(page.getByRole('link', { name: /Arc Movie/ }).first()).toBeVisible();
    const lateDone = page.waitForResponse(r => r.url().includes('q=LateOldQuery'));
    releaseLate(); await lateDone;
    await expect(page.getByRole('heading', { name: 'Results for “Arc”' })).toBeVisible();
    await expect(page.getByRole('link', { name: /Arc Movie/ }).first()).toBeVisible();
  } finally { releaseLate(); }
  await page.unroute('**/api/search?*');
  let fail = true;
  await page.route('**/api/search?*', route => fail ? route.fulfill({ status: 503, json: { error: 'Search retry fixture' } }) : route.continue());
  await page.locator('aside input[placeholder="Search"]').fill('Retry fixture');
  await expect(page.getByText('Search retry fixture', { exact: true })).toBeVisible();
  fail = false;
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'No results', exact: true })).toBeVisible();
});

test('review: phone filters wrap and empty results offer visible recovery preserving sort', async ({ page, media }, info) => {
  await setup(page);
  const lib = await addLibrary(page, 'Phone filters', media.movies);
  await page.setViewportSize({ width: 390, height: 900 });
  await page.goto(`/library/${lib.id}?genre=NoSuchGenre&decade=1990&language=fra&sort=year&reverse=1&min_runtime=150`);
  await expect(page.getByRole('heading', { name: 'Nothing matches', exact: true })).toBeVisible();
  // Each filter that's on shows as a chip beside Filters; the empty result offers Clear filters.
  await expect(page.getByRole('button', { name: 'Filters, 4 on', exact: true })).toBeVisible();
  const clear = page.getByRole('button', { name: 'Clear filters', exact: true });
  await expect(clear).toHaveCount(1);
  for (const chip of await page.getByRole('button', { name: /^Remove filter: / }).all()) {
    const box = await chip.boundingBox();
    expect(box!.x + box!.width).toBeLessThanOrEqual(390);
  }
  for (const button of await clear.all()) {
    await expect(button).toBeVisible();
    const box = await button.boundingBox();
    expect(box?.x).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(390);
  }
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  await page.getByRole('button', { name: 'Remove filter: NoSuchGenre', exact: true }).click();
  await expect(page).not.toHaveURL(/genre=/);
  expect(new URL(page.url()).searchParams.get('min_runtime')).toBe('150');
  await page.screenshot({ path: info.outputPath('phone-filter-controls.png') });
  await clear.last().scrollIntoViewIfNeeded();
  await expect(clear.last()).toBeInViewport();
  await page.screenshot({ path: info.outputPath('phone-filter-recovery.png') });
  await clear.last().focus();
  await page.keyboard.press('Enter');
  await expect(page.getByRole('link', { name: /Arc Movie/ }).first()).toBeVisible();
  const url = new URL(page.url());
  expect(url.searchParams.get('sort')).toBe('year');
  expect(url.searchParams.get('reverse')).toBe('1');
  expect([...url.searchParams.keys()].sort()).toEqual(['reverse', 'sort']);
});

test('review: manual additions only offer editable playlists', async ({ page, media }) => {
  await setup(page);
  const lib = await addLibrary(page, 'Playlist movies', media.movies);
  expect((await page.request.post('/api/playlists', { data: { title: 'Rule based list', filter: { library_id: lib.id, unwatched: true } } })).ok()).toBeTruthy();
  expect((await page.request.post('/api/playlists', { data: { title: 'Manual list' } })).ok()).toBeTruthy();
  await page.goto(`/library/${lib.id}`);
  await page.getByRole('link', { name: /Arc Movie/ }).first().click({ position: { x: 12, y: 60 } }); // beside the poster's hover buttons
  await page.getByRole('button', { name: 'More', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Add to Playlist…', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'Add to Playlist', exact: true });
  await expect(dialog.getByRole('button', { name: /Manual list/ })).toBeVisible();
  await expect(dialog.getByRole('button', { name: /Rule based list/ })).toHaveCount(0);
  await dialog.getByRole('button', { name: /Manual list/ }).click();
  await expect(dialog).not.toBeVisible();
});

test('review: late pre-roll lookup cannot replace the latest Play choice', async ({ page, media }) => {
  await setup(page);
  const lib = await addLibrary(page, 'Play choices', media.movies);
  await page.goto(`/library/${lib.id}`);
  await page.getByRole('link', { name: /Arc Movie/ }).first().click({ position: { x: 12, y: 60 } }); // beside the poster's hover buttons
  let release!: () => void;
  const held = new Promise<void>(resolve => { release = resolve; });
  let first = true;
  await page.route('**/api/items/*/preroll', async route => {
    if (!first) return route.continue();
    first = false;
    const response = await route.fetch();
    await held;
    await route.fulfill({ response });
  });
  try {
    await page.getByRole('button', { name: 'Play', exact: true }).click();
    await expect.poll(() => first).toBe(false);
    await page.locator('aside').getByRole('link', { name: 'Play choices', exact: true }).click();
    await page.getByRole('link', { name: /Unmatched Mystery/ }).first().click({ position: { x: 12, y: 60 } }); // beside the poster's hover buttons
    await page.getByRole('button', { name: 'Play', exact: true }).click();
    await expect(page.locator('video')).toBeVisible();
    const late = page.waitForResponse(r => r.url().endsWith('/preroll') && r.ok());
    release();
    await late;
    await page.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
    await expect(page.locator('video').locator('..').getByText('Unmatched Mystery', { exact: true })).toBeVisible();
  } finally { release(); }
  await page.getByRole('button', { name: 'Close player', exact: true }).click();
});

test('review: changing libraries exits bulk selection', async ({ page, media }) => {
  await setup(page);
  const first = await addLibrary(page, 'First library', media.movies);
  await addLibrary(page, 'Second library', media.empty);
  await page.goto(`/library/${first.id}`);
  await page.getByRole('button', { name: 'More for First library', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Select titles', exact: true }).click();
  await page.getByRole('button', { name: 'Select loaded (2)', exact: true }).click();
  await expect(page.getByText('2 selected', { exact: true })).toBeVisible();
  await page.locator('aside').getByRole('link', { name: 'Second library', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Stop choosing', exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: 'More for Second library', exact: true }).click();
  await expect(page.getByRole('menuitem', { name: 'Select titles', exact: true })).toBeEnabled();
  await page.keyboard.press('Escape');
});

test('library ⋯ manages the library on every tab, owner only, without removing it', async ({ page, media }, info) => {
  await setup(page);
  const lib = await addLibrary(page, 'Menu library', media.movies);
  await page.goto(`/library/${lib.id}`);
  const more = page.getByRole('button', { name: 'More for Menu library', exact: true });
  const item = (name: string) => page.getByRole('menuitem', { name, exact: true });
  await more.click();
  for (const name of ['Select titles', 'Scan library files', 'Edit library…', 'Refresh all metadata…', 'Reanalyze files…', 'Empty trash…', 'Manage access…'])
    await expect(item(name)).toBeVisible();
  await expect(page.getByRole('menuitem', { name: /Remove|Delete/ })).toHaveCount(0);
  await page.screenshot({ path: info.outputPath('library-menu.png') });
  await item('Edit library…').click();
  const editor = page.getByRole('dialog', { name: 'Edit Menu library', exact: true });
  await expect(editor).toBeVisible();
  await editor.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(editor).not.toBeVisible();
  // The costly repairs ask first, as on Manage Libraries.
  await more.click();
  await item('Refresh all metadata…').click();
  const refresh = page.getByRole('alertdialog', { name: 'Refresh all metadata?', exact: true });
  await expect(refresh).toContainText('Menu library');
  await page.keyboard.press('Escape');
  await expect(refresh).not.toBeVisible();
  await more.click();
  await item('Empty trash…').click();
  const trash = page.getByRole('alertdialog', { name: 'Remove titles whose files are gone?', exact: true });
  await expect(trash).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(trash).not.toBeVisible();
  // The other tabs have the same menu, without choosing titles.
  await page.getByRole('tab', { name: 'Recommended', exact: true }).click();
  await more.click();
  await expect(item('Edit library…')).toBeVisible();
  await expect(item('Select titles')).toHaveCount(0);
  await item('Manage access…').click();
  await expect(page).toHaveURL(/\/profiles$/);
  // A household profile has no library ⋯ on any tab.
  const viewer = await (await page.request.post('/api/profiles', { data: { name: 'Menu viewer', color: 'blue' } })).json();
  expect((await page.request.post('/api/auth/profile', { data: { id: viewer.id } })).ok()).toBe(true);
  for (const view of ['', '?view=recommended']) {
    await page.goto(`/library/${lib.id}${view}`);
    await expect(page.getByRole('heading', { name: 'Menu library', exact: true })).toBeVisible();
    await expect(more).toHaveCount(0);
  }
});

test('restoring a backup immediately restores running settings and access controls', async ({ page, media }) => {
  void media;
  await setup(page);
  const before = { region: 'GB', local_bypass: false, max_transcodes: 1, scan_automatic: false, scan_partial: false, scan_on_start: false, scan_interval_hours: 0, preview_mode: 'off', detection_mode: 'off', analysis_enabled: false, metadata_refresh_enabled: false, auto_backups_enabled: false };
  expect((await page.request.patch('/api/settings', { data: before })).ok()).toBeTruthy();
  const backup = await page.request.post('/api/backups');
  expect(backup.ok()).toBeTruthy();
  const { name } = await backup.json();
  expect((await page.request.patch('/api/settings', {
    data: { region: 'US', local_bypass: true, max_transcodes: 3, scan_automatic: true, scan_partial: true, scan_on_start: true, scan_interval_hours: 6 },
  })).ok()).toBeTruthy();
  const changed = await (await page.request.get('/api/settings')).json();
  expect(changed).toMatchObject({ region: 'US', local_bypass: true, max_transcodes: 3 });
  const restored = await page.request.post(`/api/backups/${name}/restore`);
  expect(restored.ok(), await restored.text()).toBeTruthy();
  const settings = await (await page.request.get('/api/settings')).json();
  expect(settings).toMatchObject(before);
  await page.reload();
  expect((await (await page.request.get('/api/auth/status')).json()).bypass).toBe(false);
});

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

for (const next of ['French', 'Off'] as const) {
  test(`late subtitle response cannot replace ${next}`, async ({ page, media }, testInfo) => {
    await setup(page);
    await addLibrary(page, 'Subtitle fixtures', media.movies);
    await page.locator('aside').getByRole('link', { name: 'Subtitle fixtures', exact: true }).click();
    await page.getByRole('link', { name: /Arc Movie/ }).first().click({ position: { x: 12, y: 60 } }); // beside the poster's hover buttons
    const cdp = await page.context().newCDPSession(page);
    await cdp.send('Emulation.setFocusEmulationEnabled', { enabled: true });
    await page.getByRole('button', { name: 'Play', exact: true }).click();
    await expect.poll(() => page.locator('video').evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(0);
    let release!: () => void;
    const held = new Promise<void>(resolve => { release = resolve; });
    let first = true;
    await page.route('**/api/items/*/subtitles/*', async route => {
      if (!first) return route.continue();
      first = false;
      const response = await route.fetch();
      await held;
      await route.fulfill({ response });
    });
    try {
      await page.getByRole('button', { name: 'Subtitles (C)', exact: true }).click();
      await page.getByRole('menuitem', { name: /^English/ }).click();
      await expect.poll(() => first).toBe(false);
      await page.getByRole('button', { name: 'Subtitles (C)', exact: true }).click();
      await page.getByRole('menuitem', { name: next === 'Off' ? 'Off' : /^French/ }).click();
      if (next === 'French') await expect(page.locator('[data-subtitles]')).toHaveText('French fixture subtitle');
      const late = page.waitForResponse(r => r.url().includes('/subtitles/') && r.ok());
      release();
      await late;
      // Wait until the browser has processed the delivered response and a cue animation frame.
      await page.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
      if (next === 'French') await expect(page.locator('[data-subtitles]')).toHaveText('French fixture subtitle');
      else await expect(page.locator('[data-subtitles]')).toHaveCount(0);
      await page.screenshot({ path: testInfo.outputPath('subtitle-selection.png') });
    } finally {
      release();
    }
    await page.getByRole('button', { name: 'Close player', exact: true }).click();
  });
}

test('removing a library labels its safety backup correctly', async ({ page, media }) => {
  await setup(page);
  const lib = await addLibrary(page, 'Temporary library', media.movies);
  expect((await page.request.delete(`/api/libraries/${lib.id}`)).ok()).toBeTruthy();
  const backups = await (await page.request.get('/api/backups')).json();
  expect(backups.backups.some((b: { reason: string }) => b.reason === 'before-delete')).toBe(true);
  await page.goto('/backups');
  await expect(page.getByText(/Saved before a removal/)).toBeVisible();
  await expect(page.getByText(/Backed up by you/)).toHaveCount(0);
});

for (const method of ['remux', 'transcode'] as const) {
  test(`real ${method} stream plays, seeks and resumes`, async ({ page, media }) => {
    await setup(page);
    expect((await page.request.patch('/api/settings', { data: { hw_accel: false, max_transcodes: 1 } })).ok()).toBeTruthy();
    await addLibrary(page, 'Playback fixtures', media.playback);
    await page.locator('aside').getByRole('link', { name: 'Playback fixtures', exact: true }).click();
    await page.getByRole('link', { name: method === 'remux' ? /Remux Movie/ : /Transcode Movie/ }).first().click({ position: { x: 12, y: 60 } }); // beside the poster's hover buttons
    const cdp = await page.context().newCDPSession(page);
    await cdp.send('Emulation.setFocusEmulationEnabled', { enabled: true });
    const plan = page.waitForResponse(r => r.url().endsWith('/playback') && r.ok());
    await page.getByRole('button', { name: 'Play', exact: true }).click();
    expect((await (await plan).json()).method).toBe(method);
    const video = page.locator('video');
    await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(1);
    for (let i = 0; i < 7; i++) await page.keyboard.press('l');
    // Converted streams have a relative video clock; the player's position includes offset_ms.
    await expect.poll(async () => Number(await page.getByRole('slider', { name: 'Position' }).getAttribute('aria-valuenow'))).toBeGreaterThan(65);
    await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.readyState)).toBeGreaterThanOrEqual(2);
    const saved = page.waitForResponse(r => r.url().endsWith('/progress') && r.request().method() === 'PUT'
      && r.request().postDataJSON()?.state === 'stopped' && r.ok());
    await page.getByRole('button', { name: 'Close player', exact: true }).click();
    expect((await (await saved).json()).position_ms).toBeGreaterThan(65_000);
    await page.reload();
    await page.getByRole('button', { name: /^Resume/ }).click();
    await expect.poll(async () => Number(await page.getByRole('slider', { name: 'Position' }).getAttribute('aria-valuenow'))).toBeGreaterThan(65);
    await expect.poll(() => page.locator('video').evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(1);
    await page.getByRole('button', { name: 'Close player', exact: true }).click();
  });
}

test('optimized playback respects preferred and explicit audio and identifies served audio', async ({ page, media }, info) => {
  await setup(page);
  await addLibrary(page, 'Audio fixtures', media.audio);
  const lib = (await (await page.request.get('/api/libraries')).json()).find((l: { name: string }) => l.name === 'Audio fixtures');
  const item = (await (await page.request.get(`/api/items?library_id=${lib.id}`)).json()).items[0];
  expect((await page.request.post('/api/optimize', { data: { item_ids: [item.id], quality: 720 } })).ok()).toBeTruthy();
  await expect.poll(async () => (await (await page.request.get('/api/optimized')).json()).versions[0]?.status, { timeout: 30_000 }).toBe('done');
  expect((await page.request.patch('/api/me/prefs', { data: { audio_language: 'fra' } })).ok()).toBeTruthy();
  await page.goto(`/movie/${item.id}`);
  const cdp = await page.context().newCDPSession(page);
  await cdp.send('Emulation.setFocusEmulationEnabled', { enabled: true });
  const frenchResponse = page.waitForResponse(r => r.url().endsWith('/playback') && r.ok());
  await page.getByRole('button', { name: 'Play', exact: true }).click();
  const french = await (await frenchResponse).json();
  expect(french.audio_stream).toBe(2);
  expect(french.method).toBe('remux');
  expect(french.optimized).toBeUndefined();
  const video = page.locator('video');
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(1);
  await page.getByRole('button', { name: 'Audio', exact: true }).click();
  const englishResponse = page.waitForResponse(r => r.url().endsWith('/playback') && r.ok());
  await page.getByRole('menuitem', { name: /English/ }).click();
  const english = await (await englishResponse).json();
  expect(english.audio_stream).toBe(1);
  expect(english.optimized).toMatchObject({ quality: 720, source_audio_stream: 1, audio: { index: 1, language: 'eng', codec: 'aac', channels: 2 } });
  expect(english.url).toContain('optimized=720');
  expect(english.optimized.bitrate).toBeGreaterThan(0);
  expect(english.optimized.duration_ms).toBeGreaterThan(119_000);
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(1);
  await page.getByRole('button', { name: 'Playback info', exact: true }).click();
  await expect(page.getByText('Optimized Direct Play', { exact: true })).toBeVisible();
  await expect(page.getByText(/optimized 720p copy.*English.*AAC.*Stereo/)).toBeVisible();
  await page.screenshot({ path: info.outputPath('optimized-audio-info.png') });
  await page.keyboard.press('Escape');
  await page.getByRole('button', { name: 'Audio', exact: true }).click();
  const backToFrench = page.waitForResponse(r => r.url().endsWith('/playback') && r.ok());
  await page.getByRole('menuitem', { name: /French/ }).click();
  expect((await (await backToFrench).json()).audio_stream).toBe(2);
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(1);
  await page.getByRole('button', { name: 'Close player', exact: true }).click();
});

test('optimized bitrate budget uses measured output and falls back to real conversion', async ({ page, media }, info) => {
  await setup(page);
  const lib = await addLibrary(page, 'Budget fixtures', media.budget);
  const item = (await (await page.request.get(`/api/items?library_id=${lib.id}`)).json()).items[0];
  expect((await page.request.post('/api/optimize', { data: { item_ids: [item.id], quality: 720 } })).ok()).toBeTruthy();
  await expect.poll(async () => (await (await page.request.get('/api/optimized')).json()).versions[0]?.status, { timeout: 45_000 }).toBe('done');
  const caps = { containers: ['mp4'], video: ['h264'], audio: ['aac'], max_height: 720 };
  const unlimitedResponse = await page.request.post(`/api/items/${item.id}/playback`, { data: { caps } });
  expect(unlimitedResponse.ok()).toBeTruthy();
  const unlimited = await unlimitedResponse.json();
  await info.attach('measured-optimized-copy', { body: JSON.stringify(unlimited.optimized, null, 2), contentType: 'application/json' });
  expect(unlimited.optimized.bitrate).toBeGreaterThan(2_000_000);
  expect(unlimited.optimized.duration_ms).toBeGreaterThanOrEqual(5900);
  const limitedResponse = await page.request.post(`/api/items/${item.id}/playback`, { data: { caps, max_kbps: 2000 } });
  expect(limitedResponse.ok()).toBeTruthy();
  const limited = await limitedResponse.json();
  expect(limited.method).toBe('transcode');
  expect(limited.optimized).toBeUndefined();
  expect(limited.hls).toBe(true);
  await expect.poll(async () => (await page.request.get(limited.url)).status()).toBe(200);
  expect((await page.request.delete(`/api/transcode/${limited.session_id}`)).ok()).toBeTruthy();
});

async function libraryForm(page: Page, name: string, path: string, shows = false) {
  await page.goto('/libraries');
  await page.getByRole('button', { name: 'Add Library', exact: true }).click();
  const dialog = page.getByRole('dialog');
  if (shows) await dialog.getByRole('button', { name: 'TV Shows', exact: true }).click();
  await dialog.getByRole('textbox').first().fill(name);
  await dialog.getByRole('button', { name: 'Next', exact: true }).click();
  const folder = dialog.getByPlaceholder('Choose a drive, or type a folder path');
  await folder.fill(path);
  const listed = page.waitForResponse(r => r.url().includes('/api/folders?') && r.ok());
  await folder.press('Enter');
  await listed;
  await dialog.getByRole('button', { name: 'Use This Folder', exact: true }).click();
  return dialog;
}

async function preview(page: Page, name: string, path: string, shows = false) {
  const dialog = await libraryForm(page, name, path, shows);
  await dialog.getByRole('tab', { name: 'Advanced' }).click();
  const started = page.waitForResponse(r => r.url().endsWith('/api/library-previews') && r.request().method() === 'POST' && r.ok());
  await dialog.getByRole('button', { name: 'Preview folders', exact: true }).click();
  const { id } = await (await started).json();
  await expect(page.getByRole('dialog', { name: `Preview: ${name}`, exact: true })).toBeVisible();
  await expect.poll(async () => (await (await page.request.get(`/api/library-previews/${id}`)).json()).stage).toBe('done');
  return dialog;
}

async function addLibrary(page: Page, name: string, path: string, shows = false) {
  const dialog = await libraryForm(page, name, path, shows);
  const created = page.waitForResponse(r => r.url().endsWith('/api/libraries') && r.request().method() === 'POST' && r.ok());
  await dialog.getByRole('button', { name: 'Add Library', exact: true }).click();
  const library = await (await created).json();
  await expect(dialog).not.toBeVisible();
  await expect.poll(async () => (await (await page.request.get(`/api/libraries/${library.id}/scan`)).json()).running,
    { timeout: 30_000 }).toBe(false);
  return library as { id: number };
}

async function profileMenu(page: Page, name: string) {
  await page.locator('aside').getByRole('button', { name: new RegExp(name) }).click();
}

test('setup validation, wrong password, sign-out and fresh sign-in', async ({ page, media }) => {
  void media;
  await page.goto('/');
  await page.getByLabel('Your name', { exact: true }).fill('Admin');
  await page.getByLabel('Password', { exact: true }).fill('short');
  await page.getByLabel('Password again', { exact: true }).fill('different');
  await expect(page.getByRole('button', { name: 'Continue', exact: true })).toBeDisabled();
  await setup(page);
  await profileMenu(page, 'Admin');
  await page.getByRole('menuitem', { name: 'Sign Out', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Sign in', exact: true })).toBeVisible();
  await page.getByLabel('Password', { exact: true }).fill('incorrect-password');
  await page.getByRole('button', { name: 'Sign In', exact: true }).click();
  await expect(page.getByText("that password isn't right", { exact: true })).toBeVisible();
  await page.getByLabel('Password', { exact: true }).fill(password);
  await page.getByRole('button', { name: 'Sign In', exact: true }).click();
  await expect(page.getByRole('heading', { name: "Who's watching?" })).toBeVisible();
  await page.getByRole('button', { name: 'Admin', exact: true }).click();
  await expect(page.locator('aside')).toBeVisible();
});

test('background movie import and manually fixing an unmatched title persist', async ({ page, media }) => {
  await setup(page);
  const lib = await addLibrary(page, 'Test Movies', media.movies);
  await page.locator('aside').getByRole('link', { name: 'Test Movies', exact: true }).click();
  await page.getByRole('link', { name: /Unmatched Mystery/ }).first().click({ position: { x: 12, y: 60 } }); // beside the poster's hover buttons
  await expect(page.getByRole('heading', { name: 'Unmatched Mystery', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Identify it…', exact: true }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Title', { exact: true }).fill('Resolved Movie');
  await dialog.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(dialog.getByText('Resolved Movie', { exact: false }).last()).toBeVisible();
  await dialog.getByRole('button', { name: 'Use This', exact: true }).click();
  await expect(dialog).not.toBeVisible();
  await expect(page.getByRole('heading', { name: 'Resolved Movie', exact: true })).toBeVisible();
  await page.reload();
  await expect(page.getByRole('heading', { name: 'Resolved Movie', exact: true })).toBeVisible();
  const items = await (await page.request.get(`/api/items?library_id=${lib.id}`)).json();
  expect(items.items.map((i: { title: string }) => i.title)).toEqual(expect.arrayContaining(['Arc Movie', 'Resolved Movie']));
});

test('empty previews and failed preview requests leave the app usable', async ({ page, media }) => {
  await setup(page);
  const dialog = await preview(page, 'Empty Library', media.empty);
  await expect(dialog.getByRole('button', { name: 'Add Library', exact: true })).toBeVisible();
  await expect(dialog.getByText('0', { exact: true }).first()).toBeVisible();
  await expect(page.locator('body')).not.toBeEmpty();
  await dialog.getByRole('button', { name: 'Back', exact: true }).click();
  await dialog.getByRole('tab', { name: 'Advanced' }).click();
  await page.route('**/api/library-previews', route => route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ error: 'Fixture preview unavailable' }) }));
  await dialog.getByRole('button', { name: 'Preview folders', exact: true }).click();
  await expect(dialog.getByText('Fixture preview unavailable', { exact: true })).toBeVisible();
  await expect(dialog.getByRole('button', { name: 'Preview folders', exact: true })).toBeEnabled();
  // A failed optional preview must not prevent adding the valid folder directly.
  const created = page.waitForResponse(r => r.url().endsWith('/api/libraries') && r.request().method() === 'POST' && r.ok());
  await dialog.getByRole('button', { name: 'Add Library', exact: true }).click();
  await created;
  await expect(dialog).not.toBeVisible();
});

test('alternate episode order persists and unplaced files remain visible', async ({ page, media }) => {
  await setup(page);
  await addLibrary(page, 'Test Shows', media.shows, true);
  await page.locator('aside').getByRole('link', { name: 'Test Shows', exact: true }).click();
  await page.getByRole('link', { name: /Arc Show/ }).first().click({ position: { x: 12, y: 60 } }); // beside the poster's hover buttons
  const first = page.getByText('First Chapter', { exact: true });
  const second = page.getByText('Second Chapter', { exact: true });
  await expect(first).toBeVisible();
  await expect(second).toBeVisible();
  expect((await first.boundingBox())!.y).toBeLessThan((await second.boundingBox())!.y);
  await openOrdering(page);

  await chooseDisplay(page, /Story Order/);
  await page.getByRole('button', {name:'Save',exact:true}).click();
  await openOrdering(page);
  await expect(page.getByRole('combobox',{name:'Display order',exact:true}).locator('option:checked')).toContainText('Story Order');
  await page.getByRole('button', {name:'Cancel',exact:true}).click();
  await expect.poll(async () => (await second.boundingBox())!.y < (await first.boundingBox())!.y).toBe(true);
  await page.reload();
  await openOrdering(page);
  await expect(page.getByRole('combobox',{name:'Display order',exact:true}).locator('option:checked')).toContainText('Story Order');
  await page.getByRole('button', {name:'Cancel',exact:true}).click();
  await expect(second).toBeVisible();
  expect((await second.boundingBox())!.y).toBeLessThan((await first.boundingBox())!.y);
  await page.getByRole('button', { name: /Not in This Order/ }).click();
  await expect(page.getByText('Episode 99', { exact: true })).toBeVisible();
});

test('profile switching enforces library access and protects admin pages', async ({ page, media }) => {
  await setup(page);
  const movies = await addLibrary(page, 'Private Movies', media.movies);
  await addLibrary(page, 'Allowed Shows', media.shows, true);
  const hidden = (await (await page.request.get(`/api/items?library_id=${movies.id}`)).json()).items[0];
  await page.locator('aside').getByRole('link', { name: 'Settings', exact: true }).click();
  await page.getByRole('navigation', { name: 'Settings sections' }).getByRole('link', { name: 'Profiles', exact: true }).click();
  await page.getByRole('button', { name: 'Add Profile', exact: true }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByLabel('Name', { exact: true }).fill('Viewer');
  await dialog.getByRole('button', { name: 'Add PIN', exact: true }).click();
  await dialog.getByPlaceholder('4 digits').fill('1234');
  await dialog.getByRole('switch').last().uncheck();
  await dialog.getByRole('checkbox', { name: 'Private Movies', exact: true }).uncheck();
  await dialog.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(dialog).not.toBeVisible();
  await profileMenu(page, 'Admin');
  await page.getByRole('menuitem', { name: 'Switch Profile', exact: true }).click();
  await page.getByRole('button', { name: 'Viewer', exact: true }).click();
  const rejected = page.waitForResponse(r => r.url().endsWith('/api/auth/profile') && r.status() === 401);
  await page.getByLabel('PIN', { exact: true }).fill('0000');
  await rejected;
  await expect(page.getByText('Try again', { exact: true })).toBeVisible();
  await page.getByLabel('PIN', { exact: true }).fill('1234');
  await expect(page.locator('aside').getByRole('link', { name: 'Allowed Shows', exact: true })).toBeVisible();
  await expect(page.locator('aside').getByRole('link', { name: 'Private Movies', exact: true })).toHaveCount(0);
  expect((await page.request.get('/api/profiles')).status()).toBe(403);
  await page.goto(`/movie/${hidden.id}`);
  await expect(page.getByText('item not found', { exact: true })).toBeVisible();
  await page.goto('/profiles');
  await expect(page).toHaveURL(/\/settings$/);
  await expect(page.getByRole('navigation', { name: 'Settings sections' })).toHaveCount(0);
  await expect(page.getByRole('switch', { name: 'Auto-Play', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: /^Activity/ })).toHaveCount(0);
  for (const path of ['/dashboard', '/insights', '/health', '/libraries', '/backups']) {
    await page.goto(path);
    await expect(page).toHaveURL(/\/settings$/);
  }

});

test('real video plays, saves progress on close and resumes after reload', async ({ page, media }, testInfo) => {
  await setup(page);
  await addLibrary(page, 'Test Movies', media.movies);
  await page.locator('aside').getByRole('link', { name: 'Test Movies', exact: true }).click();
  await page.getByRole('link', { name: /Arc Movie/ }).first().click({ position: { x: 12, y: 60 } }); // beside the poster's hover buttons
  const cdp = await page.context().newCDPSession(page);
  await cdp.send('Emulation.setFocusEmulationEnabled', { enabled: true });
  const plan = page.waitForResponse(r => r.url().endsWith('/playback') && r.ok());
  await page.getByRole('button', { name: 'Play', exact: true }).click();
  const decision = await (await plan).json();
  expect(decision.method).toBe('direct');
  const video = page.locator('video');
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(0);
  // Exercise the player's real seek handler, not a fabricated progress API request.
  // Arc deliberately keeps resume points only after the first minute.
  for (let i = 0; i < 7; i++) await page.keyboard.press('l');
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(65);
  const saved = page.waitForResponse(r => r.url().endsWith('/progress') && r.request().method() === 'PUT'
    && r.request().postDataJSON()?.state === 'stopped' && r.ok());
  await page.getByRole('button', { name: 'Close player', exact: true }).click();
  expect((await (await saved).json()).position_ms).toBeGreaterThan(65_000);
  await page.reload();
  const resume = page.getByRole('button', { name: /^Resume/ });
  await expect(resume).toBeVisible();
  const screenshot = testInfo.outputPath('saved-resume.png');
  await page.screenshot({ path: screenshot });
  await testInfo.attach('Saved resume point after reload', { path: screenshot, contentType: 'image/png' });
  await resume.click();
  await expect.poll(() => page.locator('video').evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(65);
  await page.getByRole('button', { name: 'Close player', exact: true }).click();
});


test('global scanning controls persist, apply live and respect startup Off', async ({ page, media }, info) => {
  await setup(page);
  await page.goto('/settings?section=library');
  const automatic = page.getByRole('switch', { name: 'Scan automatically when files change', exact: true });
  await expect(automatic).toBeChecked();
  const schedule = page.getByRole('combobox', { name: 'Full library scan', exact: true });
  await expect(schedule).toHaveValue('60');
  await page.getByText('Advanced scanning options', { exact: true }).click();
  const partial = page.getByRole('switch', { name: 'Scan only changed folders', exact: true });
  const startup = page.getByRole('switch', { name: 'Scan when Arc starts', exact: true });
  await automatic.click();
  await expect(automatic).not.toBeChecked();
  await expect(partial).toBeDisabled();
  await expect(partial).toBeChecked();
  await expect(schedule).toBeEnabled();
  await schedule.selectOption('0');
  await expect(startup).toBeEnabled();
  await startup.click();
  await expect(startup).not.toBeChecked();
  await expect(page.getByText(/Libraries update when you scan them or change their folders/)).toBeVisible();
  await page.reload();
  await expect(automatic).not.toBeChecked();
  await expect(schedule).toHaveValue('0');
  await expect(page.getByRole('switch', { name: 'Scan when Arc starts', exact: true })).not.toBeVisible();
  await page.getByText('Advanced scanning options', { exact: true }).click();
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await schedule.scrollIntoViewIfNeeded();
    await page.screenshot({ path: info.outputPath('scanning-' + width + '.png') });
  }
  await page.setViewportSize({ width: 1280, height: 720 });
  // A failed save leaves the saved value intact and supports retry.
  await page.route('**/api/settings', route => route.request().method() === 'PATCH' ? route.fulfill({ status: 503, json: { error: 'Fixture settings unavailable' } }) : route.continue());
  await schedule.selectOption('360');
  await expect(page.getByText('Fixture settings unavailable', { exact: true })).toBeVisible();
  await expect(schedule).toHaveValue('0');
  await page.unroute('**/api/settings');
  await schedule.selectOption('360');
  await expect(schedule).toBeEnabled();
  await schedule.selectOption('0');
  await expect(schedule).toBeEnabled();
  const created = await page.request.post('/api/libraries', { data: { name: 'Manual scanning', kind: 'movies', paths: [media.empty] } });
  expect(created.ok(), await created.text()).toBe(true);
  const lib = await created.json();
  await expect.poll(async () => (await (await page.request.get('/api/libraries/' + lib.id + '/scan')).json()).running).toBe(false);
  expect((await (await page.request.get('/api/libraries/' + lib.id + '/scan')).json()).started_at).toBeTruthy();
  expect((await fetch('http://127.0.0.1:18200/restart', { method: 'POST' })).ok).toBe(true);
  const scan = await (await page.request.get('/api/libraries/' + lib.id + '/scan')).json();
  expect(scan.running).toBe(false);
  expect(scan.started_at).toBeUndefined();
  expect((await fetch('http://127.0.0.1:18200/add-unwatched-file', { method: 'POST' })).ok).toBe(true);
  // Wait longer than change settling to catch accidentally active notifications.
  await page.waitForTimeout(12_000);
  expect((await (await page.request.get('/api/items?library_id=' + lib.id)).json()).total).toBe(0);
  expect((await page.request.post('/api/libraries/' + lib.id + '/scan')).ok()).toBe(true);
  await expect.poll(async () => (await (await page.request.get('/api/libraries/' + lib.id + '/scan')).json()).running).toBe(false);
  expect((await (await page.request.get('/api/items?library_id=' + lib.id)).json()).total).toBe(1);
  expect((await page.request.patch('/api/settings', { data: { scan_on_start: true } })).ok()).toBe(true);
  expect((await fetch('http://127.0.0.1:18200/restart', { method: 'POST' })).ok).toBe(true);
  expect((await (await page.request.get('/api/libraries/' + lib.id + '/scan')).json()).started_at).toBeTruthy();
  // A household profile cannot see or change server discovery settings.
  const profile = await (await page.request.post('/api/profiles', { data: { name: 'Scan viewer', color: 'blue' } })).json();
  expect((await page.request.post('/api/auth/profile', { data: { id: profile.id } })).ok()).toBe(true);
  expect((await page.request.patch('/api/settings', { data: { scan_automatic: true } })).status()).toBe(403);
  await page.goto('/settings');
  await expect(page.getByRole('heading', { name: 'Library scanning', exact: true })).toHaveCount(0);
});

test('Unmatched filter: titles Arc couldn’t identify, shows with unplaced episodes, owner only', async ({ page, media }, info) => {
  await setup(page);
  const movies = await addLibrary(page, 'Unmatched movies', media.movies);
  const shows = await addLibrary(page, 'Unmatched TV', media.shows, true);
  await page.goto(`/library/${movies.id}`);
  await page.getByRole('button', { name: 'Filters', exact: true }).click();
  const unmatched = page.getByRole('switch', { name: /Only titles Arc couldn't identify \(1\)/ });
  await unmatched.click();
  await page.keyboard.press('Escape');
  await expect(page).toHaveURL(/unmatched=1/);
  await expect(page.getByRole('link', { name: /Unmatched Mystery/ })).toBeVisible();
  await expect(page.getByRole('link', { name: /Arc Movie/ })).toHaveCount(0);
  await page.screenshot({ path: info.outputPath('unmatched-filter.png') });
  await page.getByRole('button', { name: 'Remove filter: Unmatched', exact: true }).click();
  await expect(page).not.toHaveURL(/unmatched=1/);
  await expect(page.getByRole('link', { name: /Arc Movie/ }).first()).toBeVisible();
  // Arc Show is identified, but its S01E99 isn't an episode Arc knows.
  await page.goto(`/library/${shows.id}?unmatched=1`);
  await expect(page.getByRole('link', { name: /Arc Show/ }).first()).toBeVisible();
  await page.getByRole('button', { name: 'Filters, 1 on', exact: true }).click();
  await expect(page.getByText('Also shows with episodes Arc couldn\'t place.', { exact: true })).toBeVisible();
  await page.keyboard.press('Escape');
  const viewer = await (await page.request.post('/api/profiles', { data: { name: 'Filter viewer', color: 'blue' } })).json();
  expect((await page.request.post('/api/auth/profile', { data: { id: viewer.id } })).ok()).toBe(true);
  await page.goto(`/library/${movies.id}`);
  await page.getByRole('button', { name: 'Filters', exact: true }).click();
  await expect(page.getByRole('switch', { name: /Only titles I haven't watched/ })).toBeVisible();
  await expect(page.getByRole('switch', { name: /couldn't identify/ })).toHaveCount(0);
});


test('home-network settings explain bridge limits and keep the password choice', async ({ page, media }, info) => {
  void media;
  await setup(page);
  await page.route('**/api/settings', async route => {
    if (route.request().method() !== 'GET') return route.continue();
    const response = await route.fetch();
    await route.fulfill({ response, json: { ...await response.json(), automatic_home_networks: [], home_network_bridge: true } });
  });
  await page.goto('/settings?section=access');
  await expect(page.getByText(/No home network was detected automatically/)).toBeVisible();
  await expect(page.getByRole('status').filter({ hasText: /Bridge networking can hide client addresses/ })).toBeVisible();
  await expect(page.getByText(/Use host networking or a br0\/custom LAN network/)).toBeVisible();
  await expect(page.getByRole('switch', { name: 'Skip the password at home', exact: true })).toBeEnabled();
  await page.screenshot({ path: info.outputPath('bridge-home-networks.png'), fullPage: true });
});
