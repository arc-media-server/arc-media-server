import { test as base, expect, type Page } from '@playwright/test';
import { rename } from 'node:fs/promises';
import { relative, isAbsolute } from 'node:path';

type Media = { movies: string; shows: string };
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
    expect((await page.request.post('/api/auth/setup', { data: { name: 'Admin', password: 'Maintenance-fixture-123', local_bypass: false, tmdb_api_key: 'fixture-not-a-secret', region: 'GB' } })).ok()).toBeTruthy();
    expect((await page.request.patch('/api/settings', { data: { scan_automatic: false, scan_on_start: false, scan_interval_minutes: 0, preview_mode: 'off', detection_mode: 'off', analysis_enabled: false, metadata_refresh_enabled: false, auto_backups_enabled: false } })).ok()).toBeTruthy();
    await use(await reset.json());
    expect(errors).toEqual([]);
  },
});

async function library(page: Page, path: string, shows = false) {
  const response = await page.request.post('/api/libraries', { data: { name: 'Maintenance library', kind: shows ? 'shows' : 'movies', paths: [path] } });
  expect(response.ok()).toBeTruthy();
  const lib = await response.json();
  await expect.poll(async () => (await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json()).running).toBe(false);
  return lib;
}

test('maintenance: refresh all and reanalysis retain locks, file identity and resume', async ({ page, media }, info) => {
  const lib = await library(page, media.movies);
  const item = (await (await page.request.get(`/api/items?library_id=${lib.id}`)).json()).items.find((i: { title: string }) => i.title === 'Arc Movie');
  expect((await page.request.patch(`/api/items/${item.id}`, { data: { title: 'My locked title' } })).ok()).toBeTruthy();
  expect((await page.request.put(`/api/items/${item.id}/progress`, { data: { position_ms: 45000, duration_ms: 120000, state: 'stopped' } })).ok()).toBeTruthy();
  const before = await (await page.request.get(`/api/items/${item.id}`)).json();
  await page.goto('/libraries');
  const row = page.getByRole('region', { name: 'Maintenance library', exact: true });
  for (const operation of ['refresh', 'analyze'] as const) {
    await row.getByRole('button', { name: 'Library actions for Maintenance library', exact: true }).click();
    await page.getByRole('menuitem', { name: operation === 'refresh' ? 'Refresh all metadata…' : 'Reanalyze files…', exact: true }).click();
    const dialog = page.getByRole('alertdialog');
    await expect(dialog).toContainText(operation === 'refresh' ? 'including recently refreshed titles' : 'new files are found with Scan');
    await page.screenshot({ path: info.outputPath(`${operation}-scope.png`) });
    const started = page.waitForResponse(r => r.url().endsWith(`/libraries/${lib.id}/work`) && r.ok());
    await dialog.getByRole('button', { name: operation === 'refresh' ? 'Refresh all metadata' : 'Reanalyze files', exact: true }).click();
    expect((await (await started).json()).operation).toBe(operation);
    await expect(dialog).not.toBeVisible();
    await expect.poll(async () => (await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json()).running).toBe(false);
  }
  const status = await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json();
  expect(status).toMatchObject({ operation: 'analyze', done: 2 });
  expect(status.error).toBeUndefined();
  const after = await (await page.request.get(`/api/items/${item.id}`)).json();
  expect(after.title).toBe('My locked title');
  expect(after.user_data.position_ms).toBe(before.user_data.position_ms);
  expect(after.files.map((f: { id: number }) => f.id)).toEqual(before.files.map((f: { id: number }) => f.id));
  expect(after.files[0].streams.some((s: { kind: string }) => s.kind === 'video')).toBe(true);
  expect((await page.request.patch('/api/settings', { data: { tmdb_api_key: '', tvdb_api_key: '' } })).ok()).toBeTruthy();
  expect((await page.request.post(`/api/libraries/${lib.id}/work`, { data: { operation: 'refresh' } })).status()).toBe(202);
  await expect.poll(async () => (await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json()).running).toBe(false);
  await page.reload();
  await expect(row.getByRole('alert')).toContainText('failed');
});

test('maintenance: library Stop retries locally and a stale run ID cannot stop new work', async ({ page, media }, info) => {
  expect((await fetch('http://127.0.0.1:18200/metadata-hold?hold=1', { method: 'POST' })).ok).toBe(true);
  const lib = await (await page.request.post('/api/libraries', { data: { name: 'Stopping library', kind: 'movies', paths: [media.movies] } })).json();
  try {
    await page.goto('/libraries');
    const row = page.getByRole('region', { name: 'Stopping library', exact: true });
    const stop = row.getByRole('button', { name: 'Stop work in Stopping library', exact: true });
    await expect(stop).toBeVisible();
    const active = await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json();
    expect((await page.request.post(`/api/libraries/${lib.id}/work`, { data: { operation: 'analyze' } })).status()).toBe(409);
    let fail = true;
    await page.route(`**/api/libraries/${lib.id}/stop`, route => fail ? route.fulfill({ status: 503, json: { error: 'Stop fixture unavailable' } }) : route.continue());
    await page.setViewportSize({ width: 390, height: 900 });
    await stop.click();
    await expect(row.getByRole('alert')).toContainText('Stop fixture unavailable');
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: info.outputPath('stop-retry-phone.png') });
    fail = false;
    await row.getByRole('button', { name: 'Retry', exact: true }).click();
    await expect(row.getByText('Stopped. Completed changes were kept.', { exact: true })).toBeVisible();
    const stopped = await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json();
    expect(stopped).toMatchObject({ running: false, stopped: true, job_id: active.job_id });
    expect(stopped.error).toBeUndefined();
    expect((await page.request.post(`/api/libraries/${lib.id}/stop`, { data: { job_id: active.job_id } })).status()).toBe(202);
    expect((await page.request.post(`/api/libraries/${lib.id}/work`, { data: { operation: 'analyze' } })).status()).toBe(202);
    expect((await page.request.post(`/api/libraries/${lib.id}/stop`, { data: { job_id: active.job_id } })).status()).toBe(409);
    await expect.poll(async () => (await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json()).running).toBe(false);
    expect((await fetch('http://127.0.0.1:18200/restart', { method: 'POST' })).ok).toBe(true);
    const restarted = await page.request.post(`/api/libraries/${lib.id}/work`, { data: { operation: 'analyze' } });
    expect(restarted.status()).toBe(202);
    expect((await restarted.json()).job_id).not.toBe(active.job_id);
    expect((await page.request.post(`/api/libraries/${lib.id}/stop`, { data: { job_id: active.job_id } })).status()).toBe(409);
    await expect.poll(async () => (await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json()).running).toBe(false);
    await page.reload();
    await page.screenshot({ path: info.outputPath('stop-phone.png') });
  } finally { await fetch('http://127.0.0.1:18200/metadata-hold?hold=0', { method: 'POST' }); }
});

test('maintenance: Activity stops the owned refresh and reports Stopped', async ({ page, media }) => {
  const lib = await library(page, media.movies);
  expect((await fetch('http://127.0.0.1:18200/metadata-hold?hold=1', { method: 'POST' })).ok).toBe(true);
  try {
    expect((await page.request.post(`/api/libraries/${lib.id}/work`, { data: { operation: 'refresh' } })).status()).toBe(202);
    await page.goto('/dashboard');
    const stop = page.getByRole('button', { name: 'Stop Refreshing metadata in Maintenance library', exact: true });
    await expect(stop).toBeVisible();
    await stop.click();
    await expect(stop).toHaveCount(0);
    await expect(page.getByText('Stopped', { exact: true })).toBeVisible();
    const recent = (await (await page.request.get('/api/activity')).json()).recent;
    const task = recent.find((t: { title: string }) => t.title === 'Refreshing metadata in Maintenance library');
    expect(task.stopped).toBe(true);
    expect(task.error).toBeUndefined();
    expect((await page.request.post(`/api/activity/${task.id}/stop`)).status()).toBe(202);
  } finally { await fetch('http://127.0.0.1:18200/metadata-hold?hold=0', { method: 'POST' }); }
});

test('maintenance: server clock keeps server timezone, retries errors and reflects all-day changes', async ({ page, media }, info) => {
  void media;
  await page.goto('/settings?section=maintenance');
  const clock = page.getByLabel('Server maintenance clock', { exact: true });
  const actual = await (await page.request.get('/api/maintenance/window')).json();
  await expect(clock).toContainText(actual.timezone);
  let fail = false;
  await page.route('**/api/maintenance/window', route => route.fulfill(fail ? { status: 503, json: { error: 'Clock fixture unavailable' } } : { json: { now: '2026-10-25T01:30:00+01:00', local_time: '2026-10-25 01:30:00 +01:00 BST', timezone: 'Europe/London', open: false, all_day: false, next_start: '2026-10-25T23:00:00Z', next_local: '2026-10-25 23:00 +00:00 GMT' } }));
  await page.reload();
  await expect(clock).toContainText('Server time: 01:30 Europe/London');
  await expect(page.getByText(/Next maintenance window/)).toHaveCount(0);
  fail = true;
  await page.reload();
  await expect(clock.getByRole('alert')).toContainText('Clock fixture unavailable');
  fail = false;
  await clock.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(clock).toContainText('Europe/London');
  await page.unroute('**/api/maintenance/window');
  await page.getByRole('combobox', { name: 'Maintenance start', exact: true }).selectOption('6');
  await page.getByRole('combobox', { name: 'Maintenance end', exact: true }).selectOption('6');
  await expect(page.getByText('Tasks can run at any time.', { exact: true })).toBeVisible();
  await page.setViewportSize({ width: 390, height: 900 });
  await page.screenshot({ path: info.outputPath('server-clock-phone.png') });
});

for (const shows of [false, true]) {
  test(`maintenance: ${shows ? 'series' : 'movie'} viewing actions move Refresh into More`, async ({ page, media }, info) => {
    const lib = await library(page, shows ? media.shows : media.movies, shows);
    const item = (await (await page.request.get(`/api/items?library_id=${lib.id}&kind=${shows ? 'series' : 'movie'}`)).json()).items[0];
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`/${shows ? 'series' : 'movie'}/${item.id}`);
    await expect(page.getByRole('button', { name: 'Refresh', exact: true })).toHaveCount(0);
    await expect(page.getByRole('button', { name: `Queue ${item.title}`, exact: true })).toHaveAttribute('title', /^Playback queue/);
    await expect(page.getByRole('button', { name: 'Add to Playlist', exact: true }).first()).toHaveAttribute('title', 'Save to a playlist');
    await page.getByRole('button', { name: 'More', exact: true }).click();
    const response = page.waitForResponse(r => r.url().endsWith(`/${shows ? 'series' : 'items'}/${item.id}/refresh`) && r.ok());
    await page.getByRole('menuitem', { name: 'Refresh metadata', exact: true }).click();
    await response;
    // The generated movie is really read by FFprobe: its 160×90 frame is 16:9.
    if (!shows) await expect(page.getByText('160×90 · 16:9', { exact: true })).toBeVisible();
    await page.screenshot({ path: info.outputPath('title-actions-desktop.png') });
    await page.setViewportSize({ width: 390, height: 900 });
    await page.screenshot({ path: info.outputPath('title-actions-phone.png') });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  });
}

test('maintenance: ordinary profiles cannot start or stop jobs or read the server clock', async ({ page, media }) => {
  const lib = await library(page, media.movies);
  const created = await page.request.post('/api/profiles', { data: { name: 'Viewer', color: 'blue' } });
  expect(created.ok()).toBeTruthy();
  const viewer = await created.json();
  expect((await page.request.post('/api/auth/profile', { data: { id: viewer.id } })).ok()).toBeTruthy();
  expect((await page.request.post(`/api/libraries/${lib.id}/work`, { data: { operation: 'refresh' } })).status()).toBe(403);
  expect((await page.request.post(`/api/libraries/${lib.id}/stop`, { data: { job_id: 1 } })).status()).toBe(403);
  expect((await page.request.post('/api/activity/1/stop')).status()).toBe(403);
  expect((await page.request.get('/api/maintenance/window')).status()).toBe(403);
});


test('empty trash: automatic mode needs confirmation, saves Off by default and retries failures', async ({ page, media }, info) => {
  void media;
  await page.goto('/settings?section=maintenance');
  const toggle = page.getByRole('switch', { name: 'Empty trash automatically', exact: true });
  await expect(toggle).not.toBeChecked();
  await toggle.click();
  const dialog = page.getByRole('alertdialog');
  await expect(dialog).toContainText('watch history');
  await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(toggle).not.toBeChecked();
  let fail = true;
  await page.route('**/api/settings', route => route.request().method() === 'PATCH' && fail ? route.fulfill({ status: 503, json: { error: 'Settings fixture unavailable' } }) : route.continue());
  await toggle.click();
  await dialog.getByRole('button', { name: 'Enable automatic cleanup', exact: true }).click();
  await expect(dialog).toContainText('Settings fixture unavailable');
  expect((await (await page.request.get('/api/settings')).json()).empty_trash_enabled).toBe(false);
  fail = false;
  await dialog.getByRole('button', { name: 'Enable automatic cleanup', exact: true }).click();
  await expect(dialog).not.toBeVisible();
  await expect(toggle).toBeChecked();
  await page.reload();
  await expect(toggle).toBeChecked();
  await page.setViewportSize({ width: 390, height: 900 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await toggle.scrollIntoViewIfNeeded();
  await page.screenshot({ path: info.outputPath('empty-trash-maintenance-phone.png') });
  await toggle.click();
  await expect(toggle).not.toBeChecked();
  expect((await (await page.request.get('/api/settings')).json()).empty_trash_enabled).toBe(false);
});

test('empty trash: library menu shares destructive confirmation and retains it for Retry', async ({ page, media }, info) => {
  const lib = await library(page, media.movies);
  const items = (await (await page.request.get(`/api/items?library_id=${lib.id}`)).json()).items;
  const gone = await (await page.request.get(`/api/items/${items[0].id}`)).json();
  const path = gone.files[0].path as string;
  const rel = relative(media.movies, path);
  expect(rel && !rel.startsWith('..') && !isAbsolute(rel)).toBeTruthy();
  await rename(path, path + '.trash-fixture');
  try {
  expect((await page.request.post(`/api/libraries/${lib.id}/scan`)).ok()).toBeTruthy();
  await expect.poll(async () => (await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json()).running).toBe(false);
  await page.goto('/libraries');
  const row = page.getByRole('region', { name: 'Maintenance library', exact: true });
  const open = async () => {
    await row.getByRole('button', { name: 'Library actions for Maintenance library', exact: true }).click();
    await page.getByRole('menuitem', { name: 'Empty trash…', exact: true }).click();
  };
  await open();
  const dialog = page.getByRole('alertdialog');
  await expect(dialog).toContainText('Maintenance library');
  await expect(dialog).toContainText('watch history');
  await expect(dialog).toContainText('A backup is made first');
  await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
  let fail = true;
  await page.route(`**/api/libraries/${lib.id}/empty-trash`, route => fail ? route.fulfill({ status: 500, json: { error: "couldn't back up first, so nothing was removed: fixture disk full" } }) : route.continue());
  await open();
  await dialog.getByRole('button', { name: 'Remove', exact: true }).click();
  await expect(dialog).toContainText('fixture disk full');
  await page.screenshot({ path: info.outputPath('empty-trash-library-retry.png') });
  fail = false;
  await dialog.getByRole('button', { name: 'Remove', exact: true }).click();
  await expect(dialog).not.toBeVisible();
  expect((await (await page.request.get(`/api/items?library_id=${lib.id}`)).json()).items.length).toBe(1);
  expect((await page.request.get(`/api/items/${gone.id}`)).status()).toBe(404);
  const copies = await (await page.request.get('/api/backups')).json();
  expect(copies.backups.some((b: { reason: string }) => b.reason === 'before-delete')).toBe(true);
  const activity = await (await page.request.get('/api/activity')).json();
  expect(activity.recent.some((t: { kind: string; detail?: string }) => t.kind === 'empty_trash' && t.detail?.includes('Removed 1 missing titles'))).toBe(true);
  } finally { await rename(path + '.trash-fixture', path); }
});


test('upkeep: weekly tasks default On, run once manually and retain last results after restart', async ({ page, media }, info) => {
  void media;
  await page.goto('/settings?section=maintenance');
  const database = page.getByRole('listitem', { name: 'Optimize database', exact: true });
  const cache = page.getByRole('listitem', { name: 'Clean up cached files', exact: true });
  await expect(database.getByRole('switch', { name: 'Optimize database automatically', exact: true })).toBeChecked();
  await expect(cache.getByRole('switch', { name: 'Clean up cached files automatically', exact: true })).toBeChecked();
  for (const [row, kind] of [[database, 'database'], [cache, 'cache']] as const) {
    await row.getByRole('button', { name: /^Run .* now$/ }).click();
    await expect.poll(async () => (await (await page.request.get('/api/maintenance')).json()).find((t: { kind: string }) => t.kind === kind)?.last_run?.finished_at ?? 0).toBeGreaterThan(0);
    await expect(row).toContainText('Last result');
  }
  await expect(database).toContainText('integrity checked');
  await expect(cache).toContainText('Removed');
  const before = await (await page.request.get('/api/maintenance')).json();
  expect((await fetch('http://127.0.0.1:18200/restart', { method: 'POST' })).ok).toBe(true);
  await page.reload();
  await expect(database).toContainText('Last result');
  const after = await (await page.request.get('/api/maintenance')).json();
  for (const kind of ['database', 'cache']) expect(after.find((t: { kind: string }) => t.kind === kind).last_run).toEqual(before.find((t: { kind: string }) => t.kind === kind).last_run);
  await page.setViewportSize({ width: 390, height: 900 });
  await cache.scrollIntoViewIfNeeded();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: info.outputPath('upkeep-results-phone.png') });
  const toggle = cache.getByRole('switch', { name: 'Clean up cached files automatically', exact: true });
  await toggle.click();
  await expect(toggle).not.toBeChecked();
  await expect(cache.getByRole('button', { name: /^Run .* now$/ })).toBeDisabled();
  await page.reload();
  await expect(toggle).not.toBeChecked();
});

test('upkeep: failures remain visible and Run now retries without altering other tasks', async ({ page, media }, info) => {
  void media;
  await page.route('**/api/maintenance', async route => {
    const response = await route.fetch();
    const rows = await response.json();
    const row = rows.find((t: { kind: string }) => t.kind === 'database');
    Object.assign(row, { state: 'failed', detail: 'database integrity check failed', last_run: { started_at: 100, finished_at: 200, last_success_at: 0, detail: 'Database upkeep did not complete', error: 'database integrity check failed' } });
    await route.fulfill({ response, json: rows });
  });
  await page.goto('/settings?section=maintenance');
  const database = page.getByRole('listitem', { name: 'Optimize database', exact: true });
  await expect(database).toContainText('Needs attention');
  await expect(database).toContainText('database integrity check failed');
  let fail = true;
  await page.route('**/api/maintenance/database/run', route => fail ? route.fulfill({ status: 503, json: { error: 'Maintenance fixture unavailable' } }) : route.continue());
  await database.getByRole('button', { name: 'Run Optimize database now', exact: true }).click();
  await expect(page.getByRole('alert').filter({ hasText: 'Maintenance fixture unavailable' })).toBeVisible();
  await page.screenshot({ path: info.outputPath('upkeep-failure-retry.png') });
  fail = false;
  await page.unroute('**/api/maintenance');
  await page.getByRole('alert').filter({ hasText: 'Maintenance fixture unavailable' }).getByRole('button', { name: 'Retry', exact: true }).click();
  await expect.poll(async () => (await (await page.request.get('/api/maintenance')).json()).find((t: { kind: string }) => t.kind === 'database')?.last_run?.last_success_at ?? 0).toBeGreaterThan(0);
  await expect(database).toContainText('integrity checked');
  expect((await (await page.request.get('/api/settings')).json()).empty_trash_enabled).toBe(false);
});
