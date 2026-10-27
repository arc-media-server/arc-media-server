import { test as base, expect, type Page } from '@playwright/test';

type Media = { movies: string; shows: string };
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
  return (await (await page.request.get(`/api/items?library_id=${lib.id}`)).json()).items;
}

test('recovery: New for You retries unavailable news without reopening', async ({ page, media }, info) => {
  void media;
  let fail = true;
  await page.route('**/api/news', route => fail ? route.fulfill({ status: 503, json: { error: 'News fixture unavailable' } }) : route.continue());
  await page.goto('/');
  await page.getByRole('button', { name: /^New for you/ }).click();
  const popover = page.getByRole('dialog');
  await expect(popover.getByRole('alert')).toContainText('News fixture unavailable');
  await page.screenshot({ path: info.outputPath('news-unavailable.png') });
  fail = false;
  await popover.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(popover.getByText(/Nothing new yet/)).toBeVisible();
  await expect(popover.getByRole('alert')).toHaveCount(0);
});

test('recovery: stale news stays visible and is not marked seen after refresh failure', async ({ page, media }, info) => {
  const items = await addLibrary(page, media.movies);
  const item = items.find((item: { title: string }) => item.title === 'Arc Movie');
  await page.clock.install();
  let fail = false;
  let seen = 0;
  // Initial library imports are intentionally excluded from News. Inject a typed response
  // for a real generated item to isolate cached-news recovery from the discovery policy.
  await page.route('**/api/news', route => route.fulfill(fail
    ? { status: 503, json: { error: 'News refresh fixture failure' } }
    : { json: { entries: [{ item, count: 1, added_at: Math.floor(Date.now() / 1000), unread: seen === 0 }], unread: seen === 0 ? 1 : 0 } }));
  await page.route('**/api/news/seen', route => { seen++; return route.continue(); });
  await page.goto('/');
  await page.getByRole('button', { name: /^New for you/ }).click();
  const popover = page.getByRole('dialog');
  await expect(popover.getByRole('link', { name: /Arc Movie/ })).toBeVisible();
  fail = true;
  await page.clock.fastForward(5 * 60_000 + 1);
  await expect(popover.getByRole('alert')).toContainText('News refresh fixture failure');
  await expect(popover.getByText(/Showing the last loaded updates/)).toBeVisible();
  await expect(popover.getByRole('link', { name: /Arc Movie/ })).toBeVisible();
  await page.setViewportSize({ width: 390, height: 900 });
  await page.screenshot({ path: info.outputPath('news-stale-phone.png') });
  await page.keyboard.press('Escape');
  expect(seen).toBe(0);
  fail = false;
  await page.getByRole('button', { name: /^New for you/ }).click();
  await popover.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(popover.getByRole('alert')).toHaveCount(0);
  await page.keyboard.press('Escape');
  await expect.poll(() => seen).toBe(1);
});

test('recovery: playlist list failure preserves the title and draft through retry', async ({ page, media }, info) => {
  const items = await addLibrary(page, media.movies);
  await page.clock.install();
  const item = items.find((item: { title: string }) => item.title === 'Arc Movie');
  const saved = await page.request.post('/api/playlists', { data: { title: 'Saved queue', item_ids: [] } });
  expect(saved.ok()).toBeTruthy();
  const playlist = await saved.json();
  let fail = true;
  await page.route('**/api/playlists', route => route.request().method() === 'GET' && fail ? route.fulfill({ status: 503, json: { error: 'Playlists fixture unavailable' } }) : route.continue());
  await page.goto(`/movie/${item.id}`);
  const addToPlaylist = async () => {
    await page.getByRole('button', { name: 'More', exact: true }).click();
    await page.getByRole('menuitem', { name: 'Add to Playlist…', exact: true }).click();
  };
  await addToPlaylist();
  const dialog = page.getByRole('dialog', { name: 'Add to Playlist', exact: true });
  await expect(dialog.getByRole('alert')).toContainText('Playlists fixture unavailable');
  await dialog.getByPlaceholder('New playlist').fill('Keep my draft');
  await expect(dialog.getByRole('button', { name: 'Create', exact: true })).toBeDisabled();
  await expect(dialog.getByText(/Refresh your playlists before creating/)).toBeVisible();
  await page.screenshot({ path: info.outputPath('playlist-load-error.png') });
  fail = false;
  await dialog.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(dialog.getByRole('button', { name: /Saved queue/ })).toBeVisible();
  await expect(dialog.getByPlaceholder('New playlist')).toHaveValue('Keep my draft');
  await expect(dialog).toContainText('Arc Movie');
  await page.keyboard.press('Escape');
  fail = true;
  await page.clock.fastForward(31_000);
  await addToPlaylist();
  await expect(dialog.getByRole('alert')).toContainText('Playlists fixture unavailable');
  await expect(dialog.getByText(/Showing the last loaded playlists/)).toBeVisible();
  await expect(dialog.getByRole('button', { name: /Saved queue/ })).toBeVisible();
  await dialog.getByPlaceholder('New playlist').fill('Recovered draft');
  await expect(dialog.getByRole('button', { name: 'Create', exact: true })).toBeDisabled();
  fail = false;
  await dialog.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(dialog.getByRole('alert')).toHaveCount(0);
  await expect(dialog.getByPlaceholder('New playlist')).toHaveValue('Recovered draft');
  await dialog.getByRole('button', { name: /Saved queue/ }).click();
  await expect(dialog).not.toBeVisible();
  const result = await (await page.request.get(`/api/playlists/${playlist.id}`)).json();
  expect(result.entries.map((entry: { item: { id: string } }) => entry.item.id)).toContain(item.id);
});

test('recovery: episode Space and Enter activate once without nested-action playback', async ({ page, media }, info) => {
  const items = await addLibrary(page, media.shows, true);
  const show = items.find((item: { title: string }) => item.title === 'Arc Show');
  await page.goto(`/series/${show.id}`);
  let starts = 0;
  page.on('request', request => { if (request.method() === 'POST' && request.url().endsWith('/playback')) starts++; });
  const episode = page.getByRole('button', { name: /First Chapter/ }).first();
  await episode.focus();
  const before = await page.evaluate(() => window.scrollY);
  await page.keyboard.down('Space');
  await page.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
  expect(await page.evaluate(() => window.scrollY)).toBe(before);
  await page.keyboard.up('Space');
  await expect(page.locator('video')).toBeVisible();
  await expect(episode).toHaveJSProperty('tagName', 'BUTTON');
  await expect(episode.locator('button, a, input')).toHaveCount(0);
  await expect.poll(() => starts).toBe(1);
  await page.getByRole('button', { name: 'Close player', exact: true }).click();
  await expect(page.locator('video')).toHaveCount(0);
  await episode.focus();
  await page.keyboard.press('Enter');
  await expect(page.locator('video')).toBeVisible();
  await expect.poll(() => starts).toBe(2);
  await page.getByRole('button', { name: 'Close player', exact: true }).click();
  await expect(page.locator('video')).toHaveCount(0);
  const row = episode.locator('..').locator('..');
  await row.getByRole('button', { name: 'Add to Playlist', exact: true }).focus();
  await page.keyboard.press('Space');
  await expect(page.getByRole('dialog', { name: 'Add to Playlist', exact: true })).toBeVisible();
  expect(starts).toBe(2);
  await page.keyboard.press('Escape');
  await row.getByRole('button', { name: 'Mark as Watched', exact: true }).focus();
  await page.keyboard.press('Space');
  await expect(row.getByRole('button', { name: 'Mark as Unwatched', exact: true })).toBeVisible();
  expect(starts).toBe(2);
  await episode.focus();
  await page.screenshot({ path: info.outputPath('episode-row-desktop.png') });
  await page.setViewportSize({ width: 390, height: 900 });
  await episode.focus();
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  await page.screenshot({ path: info.outputPath('episode-row-phone.png') });
  await episode.click();
  await expect(page.locator('video')).toBeVisible();
  await expect.poll(() => starts).toBe(3);
  await page.screenshot({ path: info.outputPath('episode-playback.png') });
  await page.getByRole('button', { name: 'Close player', exact: true }).click();
  await expect(page.locator('video')).toHaveCount(0);
});

const panels = [
  { name: 'Devices', group: 'access', link: 'Access & devices', endpoint: 'devices', empty: 'No devices are signed in.', entry: 'Saved TV' },
  { name: 'Optimized Versions', group: 'playback', link: 'Playback & transcoding', endpoint: 'optimized', empty: 'None yet.', entry: 'Saved copy' },
  { name: 'Notifications', group: 'library', link: 'Libraries & metadata', endpoint: 'webhooks', empty: 'No notifications configured yet.', entry: 'Saved notification' },
] as const;

for (const panel of panels) {
  test(`recovery: ${panel.name} distinguishes unavailable empty and stale saved data`, async ({ page, media }, info) => {
    void media;
    await page.clock.install();
    const unix = Math.floor(Date.now() / 1000);
    const empty = panel.endpoint === 'devices' ? [] : panel.endpoint === 'optimized' ? { versions: [], total_size: 0 } : { webhooks: [], events: ['library.new'] };
    const existing = panel.endpoint === 'devices' ? [{ id: 'fixture-device', device: 'Saved TV', user_agent: '', via: 'password', profile: 'Admin', created_at: unix, last_seen_at: unix, this_device: false }]
      : panel.endpoint === 'optimized' ? { versions: [{ file_id: 91, quality: 720, item_id: 'fixture', title: 'Saved copy', status: 'done', size: 1000, created_at: unix }], total_size: 1000 }
      : { webhooks: [{ id: 'fixture-hook', name: 'Saved notification', url: 'http://127.0.0.1:18200/unused-fixture', format: 'json', events: [] }], events: ['library.new'] };
    let phase = 'failed';
    await page.route(`**/api/${panel.endpoint}`, route => route.fulfill(phase === 'failed'
      ? { status: 503, json: { error: `${panel.name} fixture unavailable` } }
      : { json: phase === 'empty' ? empty : existing }));
    await page.goto(`/settings?section=${panel.group}`);
    const section = page.locator('section').filter({ has: page.getByRole('heading', { name: panel.name, exact: true }) });
    await expect(section.getByRole('alert')).toContainText(`${panel.name} fixture unavailable`);
    if (panel.endpoint === 'webhooks') await expect(section.getByRole('button', { name: 'Add Webhook', exact: true })).toBeDisabled();
    await page.screenshot({ path: info.outputPath(`${panel.endpoint}-unavailable.png`), fullPage: true });
    phase = 'empty';
    await section.getByRole('button', { name: 'Retry', exact: true }).click();
    await expect(section.getByText(panel.empty, { exact: false })).toBeVisible();
    await expect(section.getByRole('alert')).toHaveCount(0);
    const revisit = async () => {
      await page.getByRole('link', { name: 'Your profile', exact: true }).click();
      await page.clock.fastForward(31_000);
      await page.getByRole('link', { name: panel.link, exact: true }).click();
    };
    phase = 'existing';
    await revisit();
    await expect(section.getByText(panel.entry, { exact: false })).toBeVisible();
    phase = 'failed';
    await revisit();
    await expect(section.getByRole('alert')).toContainText(`${panel.name} fixture unavailable`);
    await expect(section.getByText(panel.entry, { exact: false })).toBeVisible();
    await expect(section.getByText(/Showing the last loaded/)).toBeVisible();
    phase = 'existing';
    await section.getByRole('button', { name: 'Retry', exact: true }).click();
    await expect(section.getByRole('alert')).toHaveCount(0);
    if (panel.endpoint === 'webhooks') {
      await section.getByRole('button', { name: 'Add Webhook', exact: true }).click();
      await expect(page.getByRole('dialog', { name: 'Add a Webhook', exact: true })).toBeVisible();
      await page.getByRole('button', { name: 'Cancel', exact: true }).click();
    }
  });
}

test('recovery: device revocation failure stays on its row and retries a real session', async ({ page, media }, info) => {
  void media;
  const auth = await (await page.request.get('/api/auth/status')).json();
  const signInDevice = async (name: string) => {
    const login = await page.request.post('/api/auth/login', { data: { password, device: name } });
    expect(login.ok()).toBeTruthy();
    const { token } = await login.json();
    const headers = { Authorization: `Bearer ${token}` };
    expect((await page.request.post('/api/auth/profile', { headers, data: { id: auth.profile.id } })).ok()).toBeTruthy();
    const devices = await (await page.request.get('/api/devices')).json();
    const device = devices.find((device: { device: string }) => device.device === name);
    expect(typeof device.last_seen_at).toBe('number');
    expect(device.last_seen_at).toBeLessThan(Date.now() / 1000 + 60);
    return { headers, device };
  };
  const first = await signInDevice('Recovery TV');
  const second = await signInDevice('Other TV');
  let fail = true;
  let failSecond = true;
  let loadFail = false;
  await page.route('**/api/devices', route => loadFail ? route.fulfill({ status: 503, json: { error: 'Device list refresh fixture failure' } }) : route.continue());
  await page.route(`**/api/devices/${first.device.id}`, async route => {
    if (fail) return route.fulfill({ status: 503, json: { error: 'Device revocation fixture failure' } });
    const response = await route.fetch(); loadFail = true;
    return route.fulfill({ response });
  });
  await page.route(`**/api/devices/${second.device.id}`, route => failSecond ? route.fulfill({ status: 503, json: { error: 'Other device revocation failure' } }) : route.continue());
  await page.goto('/settings?section=access');
  const row = page.getByText('Recovery TV', { exact: true }).locator('..').locator('..');
  await row.getByRole('button', { name: 'Sign Out', exact: true }).click();
  await expect(row.getByRole('alert')).toContainText('Device revocation fixture failure');
  await expect(row.getByText('Recovery TV', { exact: true })).toBeVisible();
  expect((await page.request.get('/api/me/prefs', { headers: first.headers })).ok()).toBeTruthy();
  const otherRow = page.getByRole('group', { name: 'Device: Other TV', exact: true });
  await otherRow.getByRole('button', { name: 'Sign Out', exact: true }).click();
  await expect(otherRow.getByRole('alert')).toContainText('Other device revocation failure');
  await expect(row.getByRole('alert')).toContainText('Device revocation fixture failure');
  await page.screenshot({ path: info.outputPath('device-revocation-failure.png'), fullPage: true });
  fail = false;
  await row.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByText('Recovery TV', { exact: true })).toHaveCount(0);
  expect((await page.request.get('/api/me/prefs', { headers: first.headers })).status()).toBe(401);
  const loadError = page.getByRole('alert').filter({ hasText: 'Device list refresh fixture failure' });
  await expect(loadError).toBeVisible();
  await expect(otherRow.getByRole('alert')).toContainText('Other device revocation failure');
  loadFail = false;
  await loadError.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(loadError).toHaveCount(0);
  await expect(otherRow.getByRole('alert')).toContainText('Other device revocation failure');
  failSecond = false;
  await otherRow.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByText('Other TV', { exact: true })).toHaveCount(0);
  expect((await page.request.get('/api/me/prefs', { headers: second.headers })).status()).toBe(401);
  expect((await page.request.get('/api/devices')).ok()).toBeTruthy();
});
