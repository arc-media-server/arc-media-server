import { test as base, expect, type Page } from '@playwright/test';

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
  return (await (await page.request.get(`/api/items?library_id=${lib.id}`)).json()).items;
}


for (const shows of [false, true]) {
  test(`clarity: ${shows ? 'show' : 'movie'} library controls have labels and help in add/edit`, async ({ page, media }, info) => {
    await page.goto('/libraries');
    await page.getByRole('button', { name: 'Add Library', exact: true }).click();
    const dialog = page.getByRole('dialog');
    if (shows) await dialog.getByRole('button', { name: 'TV Shows', exact: true }).click();
    await dialog.getByRole('textbox', { name: 'Name', exact: true }).fill('Named library');
    await dialog.getByRole('tab', { name: 'Folders' }).click();
    const folder = dialog.getByPlaceholder('Choose a drive, or type a folder path');
    await folder.fill(media.empty);
    const listed = page.waitForResponse(r => r.url().includes('/api/folders?') && r.ok());
    await folder.press('Enter');
    await listed;
    await dialog.getByRole('button', { name: 'Use This Folder', exact: true }).click();
    await dialog.getByRole('tab', { name: 'Advanced' }).click();
    const labels = [shows ? 'File episode order' : 'Automatic collections', 'Collections in the library', 'Metadata language', ...(shows ? ['Metadata provider'] : [])];
    for (const label of labels) {
      const control = dialog.getByRole('combobox', { name: label, exact: true });
      await expect(control).toBeVisible();
      expect(await control.getAttribute('aria-describedby')).toBeTruthy();
      expect(await control.evaluate(element => document.getElementById(element.getAttribute('aria-describedby')!)?.textContent)).toBeTruthy();
    }
    await dialog.getByRole('tab', { name: 'General' }).click();
    await dialog.getByText('Name', { exact: true }).click();
    await expect(dialog.getByRole('textbox', { name: 'Name', exact: true })).toBeFocused();
    await dialog.getByRole('tab', { name: 'Advanced' }).click();
    const language = dialog.getByRole('combobox', { name: 'Metadata language', exact: true });
    await language.focus();
    await page.keyboard.press('Home');
    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('Tab');
    const chosen = await language.inputValue();
    if(shows) await dialog.getByRole('combobox',{name:'Metadata provider',exact:true}).selectOption('tvdb');
    const saved = page.waitForResponse(r => r.url().endsWith('/api/libraries') && r.request().method() === 'POST' && r.ok());
    await dialog.getByRole('button', { name: 'Add Library', exact: true }).click();
    const lib = await (await saved).json();
    expect(lib.metadata_language).toBe(chosen);
    if(shows) expect(lib.details_provider).toBe("tvdb");
    await expect(dialog).not.toBeVisible();
    await page.getByRole('region', { name: 'Named library', exact: true }).getByRole('button', { name: 'Edit', exact: true }).click();
    await expect(dialog.getByRole('textbox', { name: 'Name', exact: true })).toHaveValue('Named library');
    await dialog.getByRole('tab', { name: 'Advanced' }).click();
    for (const label of labels) await expect(dialog.getByRole('combobox', { name: label, exact: true })).toBeVisible();
    await expect(dialog.getByRole('combobox', { name: 'Metadata language', exact: true })).toHaveValue(chosen);
    if (shows) {
      await expect(dialog.getByRole("combobox",{name:"Metadata provider",exact:true})).toHaveValue("tvdb");
      await dialog.getByRole('combobox', { name: labels[0], exact: true }).selectOption('tvdb_absolute');
      await expect(dialog.getByText(/Episodes numbered 1, 2, 3/)).toBeVisible();
    }
    await page.setViewportSize({ width: 390, height: 900 });
    await page.screenshot({ path: info.outputPath('library-labelled-phone.png') });
    await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
  });
}

test('clarity: backup policy reflects live Off/On, windows, failures and manual snapshots', async ({ page, media }, info) => {
  void media;
  let fail = false;
  await page.route('**/api/settings', route => route.request().method() === 'GET' && fail
    ? route.fulfill({ status: 503, json: { error: 'Backup policy fixture unavailable' } }) : route.continue());
  await page.goto('/backups');
  const status = page.getByRole('region', { name: 'Automatic backup status', exact: true });
  await expect(status.getByText('Automatic backups: Off', { exact: true })).toBeVisible();
  await expect(status).toContainText('No successful backups are saved.');
  await expect(page.getByText(/minute after Arc starts/)).toHaveCount(0);
  await expect(status).toContainText('safety backups');
  await expect(page.getByRole('heading', { name: 'No backups yet' })).toBeVisible();
  await page.getByRole('button', { name: 'Back Up Now', exact: true }).click();
  await expect(page.getByText('Backed up by you', { exact: false })).toBeVisible();
  await expect(status).toContainText('Last successful backup:');
  expect((await (await page.request.get('/api/backups')).json()).backups[0].reason).toBe('manual');
  await status.getByRole('link', { name: 'Manage automatic backups' }).click();
  await expect(page).toHaveURL(/section=maintenance#scheduled-tasks/);
  await expect(page.getByRole('switch', { name: 'Automatic database backups', exact: true })).toHaveAttribute('aria-checked', 'false');
  // Use the actual Settings control, then revisit through the SPA (same query cache).
  await page.getByRole('switch', { name: 'Automatic database backups', exact: true }).click();
  await expect.poll(async () => (await (await page.request.get('/api/settings')).json()).auto_backups_enabled).toBe(true);
  await page.getByRole('link', { name: 'Backups & restore', exact: true }).click();
  await expect(status.getByText('Automatic backups: On', { exact: true })).toBeVisible();
  for (const [start, end, text] of [[23, 4, '23:00–04:00'], [6, 6, 'all day']] as const) {
    expect((await page.request.patch('/api/settings', { data: { maintenance_start: start, maintenance_end: end } })).ok()).toBeTruthy();
    await page.reload();
    await expect(status).toContainText(text);
    await expect(status).toContainText('server local time');
  }
  await page.screenshot({ path: info.outputPath('backup-policy.png') });
  fail = true;
  await page.reload();
  await expect(status.getByRole('alert')).toContainText('Backup policy fixture unavailable');
  await expect(status.getByText('Automatic backups: Off', { exact: true })).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Back Up Now', exact: true })).toBeEnabled();
  fail = false;
  await status.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(status.getByText('Automatic backups: On', { exact: true })).toBeVisible();
  const scope = page.getByRole('region', { name: 'Backup scope', exact: true });
  await expect(scope).toContainText('does not include uploaded artwork, certificates or optimized videos');
  let restores = 0;
  page.on('request', r => { if (r.method() === 'POST' && /\/api\/backups\/[^/]+\/restore$/.test(r.url())) restores++; });
  await page.getByRole('button', { name: 'Restore', exact: true }).click();
  const confirmation = page.getByRole('alertdialog', { name: 'Restore this backup?', exact: true });
  await expect(confirmation).toContainText('Database changes made since then will be undone.');
  await expect(confirmation).toContainText('Uploaded artwork, certificates and optimized videos are not restored or rolled back.');
  await page.screenshot({ path: info.outputPath('database-restore-scope.png') });
  await confirmation.getByRole('button', { name: 'Cancel', exact: true }).click();
  expect(restores, 'opening and cancelling the scope warning must not restore').toBe(0);
  await page.setViewportSize({ width: 390, height: 844 });
  await scope.scrollIntoViewIfNeeded();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: info.outputPath('backup-scope-phone.png'), fullPage: true });
});

test('clarity: bulk selection names loaded scope and does not select later pages implicitly', async ({ page, media }, info) => {
  const original = await addLibrary(page, media.movies);
  const template = original[0];
  const entries = Array.from({ length: 501 }, (_, index) => ({ ...template, id: `scope-${index}`, title: `Scope ${String(index).padStart(3, '0')}` }));
  const lib = (await (await page.request.get('/api/libraries')).json())[0];
  await page.route('**/api/items?*', route => {
    const url = new URL(route.request().url());
    if (url.searchParams.get('library_id') !== String(lib.id)) return route.continue();
    const offset = Number(url.searchParams.get('offset') || 0);
    return route.fulfill({ json: { items: entries.slice(offset, offset + 500), total: 501 } });
  });
  let submitted: string[] = [];
  // Synthetic display records never reach a real bulk mutation.
  await page.route('**/api/items/bulk', route => {
    submitted = route.request().postDataJSON().item_ids;
    return route.fulfill({ status: 204 });
  });
  await page.goto(`/library/${lib.id}?unwatched=1`);
  await expect(page.getByText('501 movies', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Select', exact: true }).click();
  await page.getByRole('button', { name: 'Select loaded (500)', exact: true }).click();
  await expect(page.getByText('500 selected', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Load more', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Watched', exact: true }).click();
  await expect.poll(() => submitted.length).toBe(500);
  expect(submitted).not.toContain('scope-500');
  await page.getByRole('button', { name: 'Load more', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Select loaded (501)', exact: true })).toBeVisible();
  await expect(page.getByText('500 selected', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Select loaded (501)', exact: true }).click();
  await expect(page.getByText('501 selected', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Select None', exact: true }).click();
  await expect(page.getByText('Choose titles', { exact: true })).toBeVisible();
  await page.setViewportSize({ width: 390, height: 900 });
  await page.screenshot({ path: info.outputPath('selection-loaded-phone.png') });
  await page.getByRole('button', { name: 'Stop choosing', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Stop choosing', exact: true })).toHaveCount(0);
});
