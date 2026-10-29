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
  const response = await page.request.post('/api/libraries', { data: { name: shows ? 'Series' : 'Movies', kind: shows ? 'shows' : 'movies', paths: [path] } });
  expect(response.ok()).toBeTruthy();
  const lib: Library = await response.json();
  await expect.poll(async () => (await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json()).running).toBe(false);
  return lib;
}


test('Empty trash runs all libraries safely and retries only incomplete results', async ({ page, media }) => {
  const first = await addLibrary(page, media.movies);
  const second = await addLibrary(page, media.shows, true);
  let calls: number[] = [];
  let retry = false;
  await page.route('**/api/libraries/*/empty-trash?safe=1', route => {
    const id = Number(new URL(route.request().url()).pathname.split('/')[3]); calls.push(id);
    return route.fulfill({ json: id === first.id || retry ? { removed: id === first.id ? 2 : 1 } : { removed: 0, skipped: 'scan_required' } });
  });
  await page.goto('/settings?section=maintenance');
  const row = page.getByRole('listitem', { name: 'Empty trash automatically', exact: true });
  await expect(row.getByRole('switch', { name: 'Empty trash automatically', exact: true })).not.toBeChecked();
  await row.getByRole('button', { name: 'Run now', exact: true }).click();
  const confirm = page.getByRole('alertdialog', { name: 'Empty trash in all libraries?', exact: true });
  await expect(confirm).toContainText('Media files are never deleted');
  await confirm.getByRole('button', { name: 'Run now', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'Empty trash in all libraries?', exact: true });
  await expect(dialog).toContainText('2 titles removed in total');
  await expect(dialog).toContainText('Skipped: run a complete scan');
  expect(calls).toEqual([first.id, second.id]);
  retry = true;
  await dialog.getByRole('button', { name: 'Retry skipped or failed libraries', exact: true }).click();
  await expect(dialog).toContainText('3 titles removed in total');
  expect(calls).toEqual([first.id, second.id, second.id]);
  await dialog.getByRole('button', { name: 'Close', exact: true }).last().click();
  expect((await (await page.request.get('/api/settings')).json()).empty_trash_enabled).toBe(false);
});
