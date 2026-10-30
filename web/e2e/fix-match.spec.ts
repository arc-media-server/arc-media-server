import { test as base, expect, type Page } from '@playwright/test';
import { copyFile, mkdir, rm } from 'node:fs/promises';
import { dirname, join } from 'node:path';

// Fix Match closes as soon as a show is chosen, as in Plex: the show's own details come first and
// its episodes are fetched by a background Activity task, with a note on the page meanwhile.
type Media = { movies: string; shows: string };
const fixture = 'http://127.0.0.1:18200';
const test = base.extend<{ media: Media; folder: string }>({
  media: async ({ page }, use) => {
    const reset = await fetch(`${fixture}/reset`, { method: 'POST' });
    expect(reset.ok).toBe(true);
    const errors: string[] = [];
    page.on('pageerror', e => errors.push(e.message));
    await page.context().route('**/*', route => {
      const url = new URL(route.request().url());
      return url.hostname === '127.0.0.1' || url.protocol === 'data:' ? route.continue() : route.abort();
    });
    expect((await page.request.post('/api/auth/setup', { data: { name: 'Admin', password: 'Fix-match-fixture-123', local_bypass: false, tmdb_api_key: 'fixture-not-a-secret', region: 'GB' } })).ok()).toBeTruthy();
    expect((await page.request.patch('/api/settings', { data: { scan_automatic: false, scan_on_start: false, scan_interval_minutes: 0, preview_mode: 'off', detection_mode: 'off', analysis_enabled: false, metadata_refresh_enabled: false, auto_backups_enabled: false } })).ok()).toBeTruthy();
    await use(await reset.json());
    for (const part of ['metadata-hold-episodes?hold=0', 'metadata-fail?part=']) await fetch(`${fixture}/${part}`, { method: 'POST' });
    expect(errors).toEqual([]);
  },
  // A show whose folder name TMDB doesn't know, so it scans unmatched: two episode files.
  folder: async ({ media }, use) => {
    const folder = join(dirname(media.shows), 'FixMatch');
    const season = join(folder, 'Mystery Programme (2020)', 'Season 01');
    await mkdir(season, { recursive: true });
    for (const n of ['01', '02']) await copyFile(join(media.shows, 'Arc Show (2020)', 'Season 01', `Arc Show - S01E${n}.mp4`), join(season, `Mystery Programme - S01E${n}.mp4`));
    try {
      await use(folder);
    } finally {
      await rm(folder, { recursive: true, force: true });
    }
  },
});

async function unmatchedShow(page: Page, folder: string) {
  const response = await page.request.post('/api/libraries', { data: { name: 'Fix Match library', kind: 'shows', paths: [folder] } });
  expect(response.ok()).toBeTruthy();
  const lib = await response.json();
  await expect.poll(async () => (await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json()).running).toBe(false);
  const show = (await (await page.request.get(`/api/items?library_id=${lib.id}&kind=series`)).json()).items[0];
  expect(show).toMatchObject({ title: 'Mystery Programme', metadata_state: 'unmatched' });
  return show as { id: string; title: string };
}

async function openFixMatch(page: Page, id: string) {
  await page.goto(`/series/${id}`);
  await page.getByRole('button', { name: 'More', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Fix Match…', exact: true }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Title').fill('Arc Show');
  await dialog.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(dialog.getByText('TMDB · 303', { exact: true })).toBeVisible();
  return dialog;
}

async function episodes(page: Page, id: string) {
  const seasons = await (await page.request.get(`/api/series/${id}/seasons`)).json();
  return seasons.seasons.flatMap((s: { episodes: { id: string; title: string; user_data?: { position_ms?: number } }[] }) => s.episodes);
}

test('fix match: the dialog closes at once and the show updates when its episodes arrive', async ({ page, media: _media, folder }, info) => {
  const show = await unmatchedShow(page, folder);
  const [first] = await episodes(page, show.id);
  expect((await page.request.put(`/api/items/${first.id}/progress`, { data: { position_ms: 75000, duration_ms: 120000, state: 'stopped' } })).ok()).toBeTruthy();
  expect((await fetch(`${fixture}/metadata-hold-episodes?hold=1`, { method: 'POST' })).ok).toBe(true);
  const dialog = await openFixMatch(page, show.id);
  const answered = page.waitForResponse(r => r.url().endsWith(`/api/items/${show.id}/match`) && r.request().method() === 'PUT');
  await dialog.getByRole('button', { name: 'Use This', exact: true }).click();
  expect((await answered).status()).toBe(200);
  // Closed while the episode lists are still held: the show's own details are already there.
  await expect(dialog).not.toBeVisible();
  await expect(page.getByRole('heading', { name: 'Arc Show', exact: true })).toBeVisible();
  const note = page.getByRole('status').filter({ hasText: 'Updating Arc Show from TMDB…' });
  await expect(note).toBeVisible();
  await page.screenshot({ path: info.outputPath('fix-match-updating.png') });
  const activity = await (await page.request.get('/api/activity')).json();
  expect(activity.running.some((t: { title: string }) => t.title.startsWith('Matching Arc Show'))).toBe(true);

  expect((await fetch(`${fixture}/metadata-hold-episodes?hold=0`, { method: 'POST' })).ok).toBe(true);
  await expect(page.getByRole('status').filter({ hasText: 'Updated Arc Show from TMDB' })).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText('First Chapter').first()).toBeVisible();
  await expect(page.getByRole('status').filter({ hasText: 'Arc Show from TMDB' })).toHaveCount(0, { timeout: 15_000 });
  // The watched position stays with the episode, which keeps its id.
  const after = await episodes(page, show.id);
  expect(after.find((e: { id: string }) => e.id === first.id)).toMatchObject({ title: 'First Chapter', user_data: { position_ms: 75000 } });
});

test('unmatch: a matched show goes back to its file names after a confirmation, keeping progress', async ({ page, media: _media, folder }, info) => {
  const show = await unmatchedShow(page, folder);
  const [first] = await episodes(page, show.id);
  expect((await page.request.put(`/api/items/${first.id}/progress`, { data: { position_ms: 75000, duration_ms: 120000, state: 'stopped' } })).ok()).toBeTruthy();
  const dialog = await openFixMatch(page, show.id);
  await dialog.getByRole('button', { name: 'Use This', exact: true }).click();
  await expect(page.getByRole('status').filter({ hasText: 'Updated Arc Show from TMDB' })).toBeVisible({ timeout: 30_000 });
  await page.getByRole('button', { name: 'More', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Unmatch…', exact: true }).click();
  const confirm = page.getByRole('alertdialog').or(page.getByRole('dialog')).filter({ hasText: 'Unmatch Arc Show?' });
  await expect(confirm).toContainText('Your files are untouched');
  await page.screenshot({ path: info.outputPath('unmatch-confirm.png') });
  await confirm.getByRole('button', { name: 'Unmatch', exact: true }).click();
  await expect(confirm).toBeHidden();
  await expect(page.getByRole('heading', { name: 'Mystery Programme', exact: true })).toBeVisible();
  // Unmatch is gone from the menu once the show is unmatched; Fix Match stays.
  await page.getByRole('button', { name: 'More', exact: true }).click();
  await expect(page.getByRole('menuitem', { name: 'Fix Match…', exact: true })).toBeVisible();
  await expect(page.getByRole('menuitem', { name: 'Unmatch…', exact: true })).toHaveCount(0);
  await page.keyboard.press('Escape');
  const after = await episodes(page, show.id);
  expect(after.some((e: { user_data?: { position_ms?: number } }) => e.user_data?.position_ms === 75000)).toBe(true);
});

test('fix match: a failure fetching the episodes shows on the page and can be retried', async ({ page, media: _media, folder }, info) => {
  const show = await unmatchedShow(page, folder);
  expect((await fetch(`${fixture}/metadata-fail?part=episodes`, { method: 'POST' })).ok).toBe(true);
  const dialog = await openFixMatch(page, show.id);
  await dialog.getByRole('button', { name: 'Use This', exact: true }).click();
  await expect(dialog).not.toBeVisible();
  const alert = page.getByRole('alert').filter({ hasText: "Couldn't fetch all of Arc Show's details from TMDB" });
  await expect(alert).toBeVisible({ timeout: 30_000 });
  await expect(alert).toContainText('400 Bad Request');
  await page.screenshot({ path: info.outputPath('fix-match-failed.png') });
  // The failure is in Activity and in the show's match explanation too.
  const activity = await (await page.request.get('/api/activity')).json();
  expect(activity.recent.find((t: { title: string }) => t.title.startsWith('Matching Arc Show'))?.error).toContain('400 Bad Request');
  const match = await (await page.request.get(`/api/items/${show.id}/match`)).json();
  expect(match.steps.at(-1).error).toContain('Refresh metadata to try again');

  expect((await fetch(`${fixture}/metadata-fail?part=`, { method: 'POST' })).ok).toBe(true);
  await alert.getByRole('button', { name: 'Try again', exact: true }).click();
  await expect(page.getByRole('status').filter({ hasText: 'Updated Arc Show from TMDB' })).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText('First Chapter').first()).toBeVisible();
});

test('fix match: a failure identifying the chosen show stays in the dialog with Retry', async ({ page, media: _media, folder }) => {
  const show = await unmatchedShow(page, folder);
  expect((await fetch(`${fixture}/metadata-fail?part=summary`, { method: 'POST' })).ok).toBe(true);
  const dialog = await openFixMatch(page, show.id);
  await dialog.getByRole('button', { name: 'Use This', exact: true }).click();
  const alert = dialog.getByRole('alert');
  await expect(alert).toContainText("couldn't fetch the new match");
  expect((await (await page.request.get(`/api/items/${show.id}`)).json()).title).toBe('Mystery Programme');
  expect((await fetch(`${fixture}/metadata-fail?part=`, { method: 'POST' })).ok).toBe(true);
  await alert.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(dialog).not.toBeVisible();
  await expect(page.getByRole('heading', { name: 'Arc Show', exact: true })).toBeVisible();
  await expect(page.getByText('First Chapter').first()).toBeVisible({ timeout: 30_000 });
});
