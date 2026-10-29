import { test as base, expect, type Page } from '@playwright/test';
import type { Library } from '../src/lib/api';

type Media = { movies: string; shows: string; audio: string; preplay: string };
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
  const lib: Library = await response.json();
  await expect.poll(async () => (await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json()).running).toBe(false);
  return lib;
}


test('Recommended editor stages, persists and resets categories independently of Home', async ({ page, media }, info) => {
  const lib = await addLibrary(page, media.movies);
  const before = await (await page.request.get('/api/me/prefs')).json();
  await page.goto('/library/' + lib.id + '?view=recommended');
  await page.getByRole('button', { name: 'Edit Recommended', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'Edit Recommended', exact: true });
  await dialog.getByRole('button', { name: 'Move Recently Added up', exact: true }).click();
  await dialog.getByRole('button', { name: 'Hide Recently Released', exact: true }).click();
  await dialog.getByRole('combobox', { name: 'Card style for Recently Added', exact: true }).selectOption('arc');
  await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
  expect((await (await page.request.get('/api/libraries/' + lib.id + '/recommended/layout')).json()).rows).toEqual([]);
  await page.getByRole('button', { name: 'Edit Recommended', exact: true }).click();
  await dialog.getByRole('button', { name: 'Move Recently Added up', exact: true }).click();
  await dialog.getByRole('button', { name: 'Hide Recently Released', exact: true }).click();
  await dialog.getByRole('combobox', { name: 'Card style for Recently Added', exact: true }).selectOption('arc');
  let fail = true;
  await page.route('**/api/libraries/' + lib.id + '/recommended/layout', route => route.request().method() === 'PUT' && fail ? route.fulfill({ status: 503, json: { error: 'Layout save unavailable' } }) : route.continue());
  await dialog.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(dialog).toContainText('Layout save unavailable');
  await expect(dialog.getByRole('combobox', { name: 'Card style for Recently Added', exact: true })).toHaveValue('arc');
  fail = false;
  await dialog.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(dialog).toHaveCount(0);
  const saved = (await (await page.request.get('/api/libraries/' + lib.id + '/recommended/layout')).json()).rows;
  expect(saved[1]).toMatchObject({ type: 'recently_added', style: 'arc' });
  expect(saved.find((r: { type: string }) => r.type === 'recently_released').hidden).toBe(true);
  const actual = (await (await page.request.get('/api/libraries/' + lib.id + '/recommended')).json()).rows;
  expect(actual.some((r: { type: string }) => r.type === 'recently_released')).toBe(false);
  expect(actual.find((r: { type: string }) => r.type === 'recently_added').style).toBe('arc');
  expect((await (await page.request.get('/api/me/prefs')).json()).home).toEqual(before.home);
  await page.reload();
  await page.getByRole('button', { name: 'Edit Recommended', exact: true }).click();
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(dialog.getByRole('combobox', { name: 'Card style for Recently Added', exact: true })).toHaveValue('arc');
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: info.outputPath('recommended-editor-phone.png') });
  await dialog.getByRole('button', { name: 'Reset', exact: true }).click();
  await dialog.getByRole('button', { name: 'Save', exact: true }).click();
  const reset = (await (await page.request.get('/api/libraries/' + lib.id + '/recommended/layout')).json()).rows;
  expect(reset.every((r: { hidden?: boolean; style?: string }) => !r.hidden && !r.style)).toBe(true);
});
