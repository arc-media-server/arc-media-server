import { test, expect, type Page } from '@playwright/test';

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
import type { Item, Library } from '../src/lib/api';
type Fixtures = { movies: string; shows: string; empty: string; playback: string };
const password = 'Arc-browser-test-123';
let media: Fixtures;

for (const shows of [false, true]) {
  test(`${shows ? 'series' : 'movie'} header keeps playback visible and expands long descriptions`, async ({ page }, info) => {
    const { item } = await seed(page, shows);
    const overview = 'A crew follows a signal through a changing landscape, discovering how the past shapes their future. '.repeat(22);
    expect((await page.request.patch('/api/items/' + item.id, { data: {
      title: 'The Last Observatory: Beyond the Northern Horizon', overview,
      locked_fields: ['title', 'overview'],
    } })).ok()).toBe(true);
    const route = `/${shows ? 'series' : 'movie'}/${item.id}`;
    for (const width of [1366, 1920, 390]) {
      await page.setViewportSize({ width, height: width === 1366 ? 768 : 900 });
      await page.goto(route);
      await expect(page.getByRole('heading', { name: 'The Last Observatory: Beyond the Northern Horizon', exact: true })).toBeVisible();
      const play = page.locator('main').getByRole('button', { name: shows ? /^Play S1/ : 'Play', exact: !shows });
      await expect(play).toBeInViewport();
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      // Missing artwork uses a real placeholder; measure its common portrait frame.
      const frame = page.locator('main [class*="aspect-[2/3]"]').first();
      if (width >= 1024) {
        const box = await frame.boundingBox();
        expect(box!.width).toBeGreaterThanOrEqual(240);
        expect(box!.width).toBeLessThanOrEqual(260);
        const heading = await page.locator('main h1').boundingBox();
        expect(Math.abs(box!.y - heading!.y)).toBeLessThan(2);
      } else {
        await expect(frame).toBeHidden();
      }
      const more = page.getByRole('button', { name: 'Show more', exact: true });
      await expect(more).toHaveAttribute('aria-expanded', 'false');
      const paragraph = page.locator('[id="' + await more.getAttribute('aria-controls') + '"]');
      const collapsed = await paragraph.evaluate(el => el.clientHeight);
      await more.focus();
      await page.keyboard.press('Enter');
      const less = page.getByRole('button', { name: 'Show less', exact: true });
      await expect(less).toHaveAttribute('aria-expanded', 'true');
      expect(await paragraph.evaluate(el => el.clientHeight)).toBeGreaterThan(collapsed);
      await less.click();
      await expect(more).toHaveAttribute('aria-expanded', 'false');
      await page.screenshot({ path: info.outputPath(`title-${width}.png`), fullPage: true });
    }
  });
}

test('Manage libraries requests individual and bulk scans with running guards and partial retry', async ({ page }, info) => {
  await expect(page.getByRole('button', { name: 'Scan all libraries', exact: true })).toHaveCount(0);
  const { lib: movies } = await seed(page);
  const others: Library[] = [];
  for (const [name, kind, path] of [['Series fixtures', 'shows', media.shows], ['Empty fixtures', 'movies', media.empty]]) {
    const result = await page.request.post('/api/libraries', { data: { name, kind, paths: [path] } });
    expect(result.ok()).toBe(true);
    const lib: Library = await result.json(); others.push(lib);
    await expect.poll(async () => (await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json()).running).toBe(false);
  }
  const [shows, empty] = others;
  const posts: number[] = [];
  page.on('request', request => {
    const match = new URL(request.url()).pathname.match(/^\/api\/libraries\/(\d+)\/scan$/);
    if (match && request.method() === 'POST') posts.push(Number(match[1]));
  });
  let release!: () => void;
  const gate = new Promise<void>(resolve => { release = resolve; });
  const movieScan = '**/api/libraries/' + movies.id + '/scan';
  await page.route(movieScan, async route => { await gate; await route.continue(); });
  await page.goto('/libraries');
  const row = page.getByRole('region', { name: movies.name, exact: true });
  const all = page.getByRole('button', { name: 'Scan all libraries', exact: true });
  // Edit sits beside the More menu, which holds Scan with the other library actions (as Plex).
  await expect(row.getByRole('button', { name: 'Scan ' + movies.name, exact: true })).toHaveCount(0);
  await expect(row.getByRole('button', { name: 'Remove ' + movies.name, exact: true })).toHaveCount(0);
  const menu = (lib: Library) => page.getByRole('region', { name: lib.name, exact: true }).getByRole('button', { name: 'Library actions for ' + lib.name, exact: true });
  await menu(movies).click();
  await page.getByRole('menuitem', { name: 'Scan library files', exact: true }).click();
  await expect(row.getByText('Requesting a scan…', { exact: true })).toBeVisible();
  await expect(all).toBeDisabled();
  await menu(movies).click();
  await expect(page.getByRole('menuitem', { name: 'Scan library files', exact: true })).toBeDisabled();
  await page.keyboard.press('Escape');
  release();
  await expect(all).toBeEnabled();
  await expect.poll(async () => (await (await page.request.get(`/api/libraries/${movies.id}/scan`)).json()).running).toBe(false);
  expect(posts).toEqual([movies.id]);
  await page.unroute(movieScan);
  await page.screenshot({ path: info.outputPath('manage-scans-desktop.png'), animations: 'disabled' });

  // Simulate another client already scanning this library; it must get no POST.
  const busy = new Set([shows.id]);
  await page.route('**/api/libraries', async route => {
    const response = await route.fetch();
    const libraries: Library[] = await response.json();
    await route.fulfill({ response, json: libraries.map(lib => busy.has(lib.id) ? { ...lib, scan: { ...lib.scan, running: true, phase: 'Checking files and titles' } } : lib) });
  });
  await page.route(movieScan, route => route.fulfill({ status: 503, json: { error: 'Fixture scan unavailable' } }));
  await page.setViewportSize({ width: 390, height: 844 });
  await page.reload();
  await menu(shows).click();
  await expect(page.getByRole('menuitem', { name: 'Scan library files', exact: true })).toBeDisabled();
  await page.keyboard.press('Escape');
  await all.click();
  await expect(page.getByText('Scan requested for 1 library. 1 library could not start. Retry below.', { exact: true })).toBeVisible();
  await expect(row.getByText('Fixture scan unavailable', { exact: true })).toBeVisible();
  expect(posts.filter(id => id === shows.id)).toHaveLength(0);
  expect(posts.filter(id => id === empty.id)).toHaveLength(1);
  expect(posts.filter(id => id === movies.id)).toHaveLength(2);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: info.outputPath('manage-scans-phone-error.png'), fullPage: true, animations: 'disabled' });
  await page.unroute(movieScan);
  await row.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(row.getByText('Fixture scan unavailable', { exact: true })).toHaveCount(0);
  await expect(all).toBeEnabled();
  expect(posts.filter(id => id === movies.id)).toHaveLength(3);
  busy.add(movies.id); busy.add(empty.id);
  await page.reload();
  await expect(all).toBeDisabled();
  await expect(page.getByText('All libraries are already scanning.', { exact: true })).toBeVisible();
});

test('management pages share Settings navigation and Activity opens Dashboard on desktop and phone', async ({ page }, info) => {
  await seed(page);
  expect((await page.request.patch('/api/settings', { data: { server_name: 'Living Room' } })).ok()).toBe(true);
  await page.route('**/api/health/libraries', route => route.fulfill({ json: { fixture: 900 } }));
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('/');
  const sidebar = page.locator('aside');
  await expect(sidebar.getByText('Manage', { exact: true })).toHaveCount(0);
  await expect(sidebar.getByRole('link', { name: 'Dashboard', exact: true })).toHaveCount(0);
  await sidebar.getByRole('link', { name: 'Settings', exact: true }).click();
  const nav = page.getByRole('navigation', { name: 'Settings sections' });
  await expect(nav.getByLabel('900 library issues')).toBeVisible();
  for (const [label, path, heading] of [
    ['Dashboard', '/dashboard', 'Living Room - Dashboard'], ['Insights', '/insights', 'Insights'],
    ['Library Health', '/health', 'Library Health'], ['Manage libraries', '/libraries', 'Libraries'],
    ['Profiles', '/profiles', 'Profiles'], ['Database and Backup', '/backups', 'Database and Backup'],
  ]) {
    const link = nav.getByRole('link', { name: new RegExp('^' + label) });
    await link.click();
    await expect(page).toHaveURL(new RegExp(path + '$'));
    await expect(link).toHaveAttribute('aria-current', 'page');
    await expect(page.getByRole('heading', { name: heading, exact: true }).first()).toBeVisible();
    await expect(sidebar.getByRole('link', { name: 'Settings', exact: true })).toHaveAttribute('aria-current', 'page');
  }
  await page.goto('/insights?tab=storage&days=90');
  await page.reload();
  await expect(nav.getByRole('link', { name: 'Insights', exact: true })).toHaveAttribute('aria-current', 'page');
  await expect(page).toHaveURL(/tab=storage&days=90/);
  await nav.getByRole('link', { name: 'Dashboard', exact: true }).click();
  await page.goBack();
  await expect(page).toHaveURL(/\/insights\?tab=storage&days=90/);
  await page.screenshot({ animations: 'disabled', path: info.outputPath('settings-management-desktop.png') });

  await page.goto('/');
  await page.getByRole('button', { name: /^Activity/ }).click();
  const dashboard = page.getByRole('link', { name: 'View Dashboard', exact: true });
  await expect(dashboard).toBeVisible();
  const activityHeading = page.getByRole('heading', { name: 'Activity', exact: true });
  await expect(activityHeading).toBeVisible();
  expect((await dashboard.boundingBox())!.y + (await dashboard.boundingBox())!.height).toBeLessThanOrEqual((await activityHeading.boundingBox())!.y);
  await page.screenshot({ animations: 'disabled', path: info.outputPath('activity-dashboard-desktop.png') });
  await dashboard.focus(); await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/dashboard$/);
  await expect(dashboard).toHaveCount(0);

  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');
  await expect(page.getByRole('button', { name: 'Manage', exact: true })).toHaveCount(0);
  await page.getByRole('link', { name: 'Settings', exact: true }).click();
  const selector = page.getByRole('combobox', { name: 'Settings section', exact: true });
  await selector.selectOption('dashboard');
  await expect(page).toHaveURL(/\/dashboard$/);
  await page.reload(); await expect(selector).toHaveValue('dashboard');
  await selector.selectOption('libraries');
  await expect(page.getByRole('heading', { name: 'Libraries', exact: true })).toBeVisible();
  await selector.selectOption('maintenance');
  await expect(page.getByRole('heading', { name: 'Scheduled tasks', exact: true })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ animations: 'disabled', path: info.outputPath('settings-management-phone.png') });
  await page.getByRole('button', { name: /^Activity/ }).click();
  await expect(dashboard).toBeVisible();
  await page.screenshot({ animations: 'disabled', path: info.outputPath('activity-dashboard-phone.png') });
  await dashboard.click();
  await expect(page).toHaveURL(/\/dashboard$/);
  await expect(dashboard).toHaveCount(0);
  await expect(selector).toHaveValue('dashboard');
});

test('settings groups expose labelled controls and persistent desktop and mobile navigation', async ({ page }, info) => {
  await seed(page);
  for (const [section, title] of [['profile', 'Your profile'], ['general', 'General'], ['library', 'Libraries & metadata'], ['playback', 'Playback & transcoding'], ['access', 'Access & devices'], ['maintenance', 'Maintenance & diagnostics']]) {
    await page.goto(section === 'profile' ? '/settings' : `/settings?section=${section}`);
    await expect(page.getByRole('heading', { name: title, exact: true, level: 1 })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Server', exact: true })).toHaveCount(0);
    await expect(page.getByText('Switches and menus save immediately. Text fields use Save.', { exact: true })).toHaveCount(0);
  }
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('/settings?section=library');
  const nav = page.getByRole('navigation', { name: 'Settings sections' });
  await expect(page.getByRole('heading', { name: 'Import from another server', exact: true })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Notifications', exact: true })).toHaveCount(0);
  await nav.getByRole('link', { name: 'General', exact: true }).click();
  await expect(page.getByRole('textbox', { name: 'Server name', exact: true })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Notifications', exact: true })).toBeVisible();
  await nav.getByRole('link', { name: 'Libraries & metadata', exact: true }).click();
  // Age-rating country is chosen per library (Edit library → Advanced), as Plex does.
  await expect(page.getByRole('combobox', { name: 'Region', exact: true })).toHaveCount(0);
  // Native links work with the keyboard and retain ordinary browser navigation semantics.
  const playback = nav.getByRole('link', { name: 'Playback & transcoding', exact: true });
  await playback.focus(); await page.keyboard.press('Enter');
  await expect(playback).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('heading', { name: 'Subtitle downloads', exact: true })).toBeVisible();
  // FFmpeg and ffprobe are found automatically here, so Media tools has nothing to show.
  await expect(page.getByRole('heading', { name: 'Media tools', exact: true })).toHaveCount(0);
  await expect(nav.getByRole('link', { name: 'Media tools', exact: true })).toHaveCount(0);
  await expect(page.getByRole('heading', { name: 'Import from another server', exact: true })).toHaveCount(0);
  await page.getByRole('heading', { name: 'Video conversion', exact: true }).scrollIntoViewIfNeeded();
  await expect(page.getByRole('heading', { name: 'Video conversion', exact: true })).toBeInViewport();
  await expect(playback).toBeInViewport();
  await expect(page.getByRole('combobox', { name: 'Simultaneous video conversions', exact: true })).toBeVisible();
  await page.screenshot({ path: info.outputPath('settings-desktop.png') });
  await page.goBack(); // Group selection participates in browser history.
  await expect(page.getByRole('heading', { name: 'Import from another server', exact: true })).toBeVisible();
  for (const group of ['profile', 'library', 'playback', 'access', 'maintenance']) {
    await page.goto('/settings?section=' + group);
    const selects = page.locator('main select:visible');
    for (const select of await selects.all()) await expect(select).toHaveAccessibleName(/.+/);
  }
  await expect(page.getByRole('link', { name: 'Database and Backup', exact: true })).toBeVisible();
  await expect(page.getByRole('link', { name: /^Library Health/ }).last()).toBeVisible();
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/settings');
  const selector = page.getByRole('combobox', { name: 'Settings section', exact: true });
  const spoiler = page.getByRole('switch', { name: 'Hide spoilers', exact: true });
  await spoiler.scrollIntoViewIfNeeded();
  await expect(selector).toBeInViewport();
  const label = page.getByText('Hide spoilers', { exact: true });
  const a = await label.boundingBox(), b = await spoiler.boundingBox();
  expect(b!.x).toBeGreaterThan(a!.x + a!.width);
  await selector.selectOption('library');
  await expect(page.getByRole('heading', { name: 'Import from another server', exact: true })).toBeVisible();
  await page.reload(); await expect(selector).toHaveValue('library');
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: info.outputPath('settings-mobile.png'), fullPage: true });
});

test('server settings keep saving, failure and retry feedback beside the edited control', async ({ page }, info) => {
  await seed(page);
  await page.goto('/settings?section=playback');
  const control = page.getByRole('combobox', { name: 'Simultaneous video conversions', exact: true });
  const row = control.locator('..');
  let release!: () => void;
  const gate = new Promise<void>(resolve => { release = resolve; });
  await page.route('**/api/settings', async route => {
    if (route.request().method() !== 'PATCH') return route.continue();
    await gate;
    return route.fulfill({ status: 503, json: { error: 'Settings save failed' } });
  });
  await control.scrollIntoViewIfNeeded();
  await control.selectOption('2');
  await expect(row.getByRole('status')).toHaveText('Saving…');
  await expect(control).toBeDisabled();
  release();
  const error = row.getByRole('alert');
  await expect(error).toContainText('Settings save failed');
  await expect(error).toBeInViewport();
  await page.screenshot({ path: info.outputPath('settings-local-error.png') });
  await page.unroute('**/api/settings');
  await error.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(row.getByRole('alert')).toHaveCount(0);
  await expect(row.getByRole('status')).toHaveCount(0);
  await expect(control).toBeEnabled();
  await expect(control).toHaveValue('2');
  await page.reload(); await expect(control).toHaveValue('2');
  // An async pre-roll save must also surface rejection rather than create an unhandled error.
  const errors: string[] = []; page.on('pageerror', error => errors.push(error.message));
  await page.route('**/api/settings', route => route.request().method() === 'PATCH'
    ? route.fulfill({ status: 503, json: { error: 'Pre-roll save failed' } }) : route.continue());
  const preroll = page.getByRole('combobox', { name: 'Pre-roll order', exact: true });
  // Leave room for the inline error before changing a control near the viewport edge.
  await preroll.evaluate(el => el.scrollIntoView({ block: 'center' }));
  await expect.poll(async () => {
    const box = await preroll.boundingBox();
    return !!box && box.y >= 0 && box.y + box.height < page.viewportSize()!.height - 150;
  }).toBe(true);
  await preroll.selectOption('all');
  await expect(page.getByRole('alert')).toContainText('Pre-roll save failed');
  await expect(page.getByRole('alert')).toBeInViewport();
  await page.unroute('**/api/settings');
  await page.getByRole('alert').getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByRole('combobox', { name: 'Pre-roll order', exact: true })).toHaveValue('all');
  await expect(page.getByRole('alert')).toHaveCount(0);
  await page.goto('/settings?section=access');
  const upload = page.getByRole('textbox', { name: 'Internet upload speed', exact: true });
  await upload.fill('25');
  await page.route('**/api/settings', route => route.request().method() === 'PATCH'
    ? route.fulfill({ status: 503, json: { error: 'Upload save failed' } }) : route.continue());
  const saveUpload = upload.locator('..').getByRole('button', { name: 'Save', exact: true });
  // Wait for page/control motion before centering. Otherwise actionability's
  // later auto-scroll can put Save at the bottom edge, below the intended setup.
  await saveUpload.click({ trial: true });
  await saveUpload.evaluate(el => el.scrollIntoView({ block: 'center' }));
  await expect.poll(async () => {
    const box = await saveUpload.boundingBox();
    return !!box && box.y >= 0 && box.y + box.height < page.viewportSize()!.height - 150;
  }, { message: 'Save has viewport room for its inline feedback before clicking' }).toBe(true);
  await saveUpload.click();
  await expect(page.getByRole('alert')).toContainText('Upload save failed');
  await expect(page.getByRole('alert')).toBeInViewport();
  await page.unroute('**/api/settings');
  await page.getByRole('alert').getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByRole('alert')).toHaveCount(0);
  await page.reload(); await expect(upload).toHaveValue('25');
  expect(errors).toEqual([]);
});

test('temporary folder shows the stored path, opens a chooser and can return to default', async ({ page }, info) => {
  await seed(page);
  await page.goto('/settings?section=playback');
  const box = page.getByRole('button', { name: 'Temporary folder', exact: true });
  await expect(box).toHaveText("Default: Arc's data folder");
  await box.click();
  await expect(box).toHaveAttribute('aria-expanded', 'true');
  await page.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(box).toHaveAttribute('aria-expanded', 'false');
  await box.click();
  const path = page.getByPlaceholder('Choose a drive, or type a folder path', { exact: true });
  await path.fill(media.movies); await path.press('Enter');
  await page.getByRole('button', { name: 'Use This Folder', exact: true }).click();
  await expect(box).toHaveText(media.movies);
  await expect(box).toHaveAttribute('aria-expanded', 'false');
  await page.reload();
  await expect(box).toHaveText(media.movies);
  await page.setViewportSize({ width: 390, height: 844 });
  await box.scrollIntoViewIfNeeded();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: info.outputPath('temporary-folder-mobile.png') });
  await box.click();
  await expect(path).toHaveValue(media.movies);
  await page.getByRole('button', { name: 'Cancel', exact: true }).click();
  await page.route('**/api/settings', route => route.request().method() === 'PATCH'
    ? route.fulfill({ status: 503, json: { error: 'Temporary folder save failed' } }) : route.continue());
  await page.getByRole('button', { name: 'Use Default', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('Temporary folder save failed');
  await expect(box).toHaveText(media.movies);
  await page.unroute('**/api/settings');
  await page.getByRole('button', { name: 'Use Default', exact: true }).click();
  await expect(box).toHaveText("Default: Arc's data folder");
  await expect(page.getByRole('alert')).toHaveCount(0);
  expect((await (await page.request.get('/api/settings')).json()).transcode_dir).toBe('');
  await page.reload(); await expect(box).toHaveText("Default: Arc's data folder");
});

test('Home distinguishes request failures from empty libraries and retries rows', async ({ page }) => {
  await seed(page);
  await page.route('**/api/libraries', r => r.fulfill({ status: 503, json: { error: 'Library service unavailable' } }));
  await page.goto('/');
  await expect(page.getByText('Library service unavailable', { exact: true })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Welcome to Arc', exact: true })).toHaveCount(0);
  await page.unroute('**/api/libraries');
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Recently Added in Audit Movies' })).toBeVisible();
  await page.route('**/api/home/continue*', r => r.fulfill({ status: 503, json: { error: 'Continue Watching unavailable' } }));
  await page.reload();
  await expect(page.getByRole('heading', { name: 'Continue Watching', exact: true })).toBeVisible();
  await expect(page.getByText('Continue Watching unavailable', { exact: true })).toBeVisible();
  await page.unroute('**/api/home/continue*');
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByText('Continue Watching unavailable', { exact: true })).toHaveCount(0);
});

test('profile preferences and display orders explain failed saves and retry', async ({ page }) => {
  const { item } = await seed(page, true);
  await page.route('**/api/me/prefs', r => r.request().method() === 'PATCH' ? r.fulfill({ status: 503, json: { error: 'Preference save failed' } }) : r.continue());
  await page.goto('/settings');
  const spoiler = page.getByRole('switch', { name: 'Hide spoilers', exact: true });
  await spoiler.click();
  await expect(page.getByText('Preference save failed', { exact: true })).toBeInViewport();
  await expect(spoiler).not.toBeChecked();
  await page.unroute('**/api/me/prefs');
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(spoiler).toBeChecked();
  await page.reload(); await expect(spoiler).toBeChecked();
  await page.route('**/display-ordering', r => r.fulfill({ status: 503, json: { error: 'Ordering save failed' } }));
  await page.goto('/series/' + item.id);
  await openOrdering(page);

  await expect(page.getByText('Applies to every profile', {exact:true})).toBeVisible();
  await chooseDisplay(page, /Story Order/);
  await page.getByRole('button',{name:'Save',exact:true}).click();
  await expect(page.getByText('Ordering save failed', { exact: true })).toBeVisible();
  await page.unroute('**/display-ordering');
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await openOrdering(page);
  await expect(page.getByRole('combobox',{name:'Display order',exact:true}).locator('option:checked')).toContainText('Story Order');
  await page.reload();
  await openOrdering(page);

  await chooseDisplay(page, /TMDB.*Aired Order/);
  await page.getByRole('button',{name:'Save',exact:true}).click();
  await openOrdering(page);
  await expect(page.getByRole('combobox',{name:'Display order',exact:true}).locator('option:checked')).toContainText('TMDB · Aired Order');
});

test('internet streaming quality is saved to the profile and only limits streams away from home', async ({ page }, info) => {
  const { item } = await seed(page);
  await page.goto('/settings');
  const quality = page.getByRole('combobox', { name: 'Internet streaming quality', exact: true });
  await expect(quality.locator('option:checked')).toHaveText('Maximum');
  await expect(page.getByText(/Applies away from home\. Compatible videos within the bitrate and resolution limit keep their original quality/)).toBeVisible();
  const saved = page.waitForResponse(r => r.url().endsWith('/api/me/prefs') && r.request().method() === 'PATCH' && r.ok());
  await quality.selectOption({ label: '4 Mbps 720p' });
  expect((await (await saved).json()).remote_quality_kbps).toBe(4000);
  await expect(quality).toBeEnabled();
  await expect(page.getByText('Saved', { exact: true })).toHaveCount(0);
  await page.screenshot({ path: info.outputPath('internet-quality-setting.png') });
  await page.reload();
  await expect(quality.locator('option:checked')).toHaveText('4 Mbps 720p');
  expect((await page.request.patch('/api/me/prefs', { data: { remote_quality_kbps: 5000 } })).status()).toBe(400);
  // The playback decision: capped away from home (through an untrusted forwarder), not at home.
  const plan = async (headers: Record<string, string>) => (await (await page.request.post('/api/items/' + item.id + '/playback', { headers, data: { caps: { containers: ['mp4', 'mkv'], video: ['h264', 'hevc'], audio: ['aac', 'ac3'] } } })).json()).server_limit_kbps;
  expect(await plan({ 'X-Forwarded-For': '203.0.113.9' })).toBe(4000);
  expect(await plan({})).toBe(0);
  await quality.selectOption({ label: 'Maximum' });
  await page.reload();
  await expect(quality.locator('option:checked')).toHaveText('Maximum');
  expect(await plan({ 'X-Forwarded-For': '203.0.113.9' })).toBe(0);
});

test('mobile profiles, settings sections and player controls remain usable', async ({ page }, info) => {
  const { item } = await seed(page);
  await page.request.post('/api/profiles', { data: { name: 'Audit Household with a long name', color: 'blue' } });
  await page.request.post('/api/profiles', { data: { name: 'Audit Friend with a long name', color: 'blue', friend: true } });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/profiles');
  await expect(page.getByText('Audit Household with a long name', { exact: true })).toBeVisible();
  const friendName = page.getByText('Audit Friend with a long name', { exact: true });
  await expect(friendName).toBeVisible();
  expect((await friendName.boundingBox())!.width).toBeGreaterThan(180);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  await page.screenshot({ path: info.outputPath('profiles-390.png'), fullPage: true });
  await page.goto('/settings');
  await expect(page.getByRole('combobox', { name: 'Preferred audio language', exact: true })).toBeVisible();
  await page.getByRole('combobox', { name: 'Preferred audio language', exact: true }).selectOption('jpn');
  await expect.poll(async () => (await (await page.request.get('/api/me/prefs')).json()).audio_language).toBe('jpn');
  for (const [tab, heading] of [['Libraries & metadata', 'Library scanning'], ['Playback & transcoding', 'Video conversion'], ['Access & devices', 'Devices'], ['Maintenance & diagnostics', 'Scheduled tasks']]) {
    await page.getByRole('combobox', { name: 'Settings section', exact: true }).selectOption({ label: tab });
    await expect(page.getByRole('heading', { name: heading, exact: true })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  }
  await page.screenshot({ path: info.outputPath('settings-390.png'), fullPage: true });
  await page.goto('/movie/' + item.id);
  await page.getByRole('button', { name: 'Play', exact: true }).click();
  await expect.poll(() => page.locator('video').evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(0);
  await page.keyboard.press('Space');
  for (const name of ['Play (Space)', 'Subtitles (C)', 'Quality', 'Speed', 'Playback queue', 'Full screen (F)']) {
    const b = await page.getByRole('button', { name, exact: true }).boundingBox();
    expect(b).not.toBeNull(); expect(b!.width).toBeGreaterThanOrEqual(44);
    expect(b!.x).toBeGreaterThanOrEqual(0); expect(b!.x + b!.width).toBeLessThanOrEqual(390);
  }
  await page.screenshot({ path: info.outputPath('player-390.png') });
  await page.getByRole('button', { name: 'Close player', exact: true }).click();
});

test('custom Home rows open complete filtered results and follow the library order', async ({ page }) => {
  const { lib, item } = await seed(page);
  expect((await page.request.patch('/api/me/prefs', { data: { home: [{ id: 'custom:ux', title: 'Short movies', filter: { library_id: lib.id, max_runtime: 90, added_within: 30, min_rating: 8, sort: 'added' } }] } })).ok()).toBe(true);
  await page.route('**/api/items?*', r => {
    const u = new URL(r.request().url());
    if (u.searchParams.get('max_runtime') !== '90') return r.continue();
    const limit = Number(u.searchParams.get('limit') || 100);
    return r.fulfill({ json: { total: 60, items: Array.from({ length: Math.min(limit, 60) }, (_, i) => ({ ...item, id: 'ux-' + i, title: 'Short movie ' + i })) } });
  });
  await page.goto('/');
  await page.getByRole('heading', { name: 'Short movies', exact: true }).getByRole('link').click();
  await expect(page).toHaveURL(/max_runtime=90/);
  await expect(page).toHaveURL(/added_within=30/);
  await expect(page).toHaveURL(/min_rating=8/);
  await expect(page.locator('main a[href^="/movie/ux-"]')).toHaveCount(60);
  await page.getByRole('button', { name: 'Filters, 3 on', exact: true }).click();
  await expect(page.getByRole('combobox', { name: 'Length', exact: true })).toHaveValue('-90');
  await page.keyboard.press('Escape');
  await page.goto('/?edit=1');
  const dialog = page.getByRole('dialog', { name: 'Edit Home', exact: true });
  // Rows follow the library order (owner, 30 October): none is moved on its own, and a filter
  // row sits under its library's newest.
  await expect(dialog.getByRole('button', { name: 'Move Watchlist up', exact: true })).toHaveCount(0);
  const titles = await dialog.locator('li p.font-medium').allTextContents();
  expect(titles.indexOf('Short movies')).toBe(titles.indexOf(`Recently Added in ${lib.name}`) + 1);
  expect(titles[0]).toBe('Continue Watching');
});

test('API-created person rows retain filters through Home and library browsing', async ({ page }) => {
  const { lib, item } = await seed(page);
  expect((await page.request.patch('/api/me/prefs', { data: { home: [{ id: 'custom:person', title: 'Unwatched actor movies', filter: { library_id: lib.id, person_id: 42, person_role: 'actor', watch_state: 'unwatched', video_codec: 'hevc', sort: 'added' } }] } })).ok()).toBe(true);
  const queries: URLSearchParams[] = [];
  await page.route('**/api/items?*', r => {
    queries.push(new URL(r.request().url()).searchParams);
    return r.fulfill({ json: { total: 1, items: [item] } });
  });
  await page.goto('/');
  const row = page.getByRole('heading', { name: 'Unwatched actor movies', exact: true });
  await expect(row).toBeVisible();
  const matches = () => queries.filter(p => p.get('person_id') === '42' && p.get('person_role') === 'actor' && p.get('unwatched') === 'true' && p.get('video_codec') === 'hevc');
  await expect.poll(() => matches().length).toBeGreaterThan(0);
  await row.getByRole('link').click();
  await expect(page).toHaveURL(/person_id=42/);
  await expect.poll(() => matches().some(p => p.get('limit') === '500')).toBe(true);
  await page.getByRole('button', { name: /^Filters,/ }).click();
  await expect(page.getByRole('switch', { name: "Only titles I haven't watched", exact: true })).toBeChecked();
  await page.keyboard.press('Escape');
  await page.getByRole('button', { name: 'Order: Newest first', exact: true }).click();
  await page.getByRole('menuitemradio', { name: 'Oldest first', exact: true }).click();
  await page.getByRole('link', { name: 'Home', exact: true }).first().click();
  await page.goto(`/library/${lib.id}`);
  await expect(page).toHaveURL(/person_id=42/);
  await expect(page).toHaveURL(/video_codec=hevc/);
  await page.getByRole('button', { name: /^Filters,/ }).click();
  await page.getByRole('button', { name: 'Clear filters', exact: true }).click();
  await expect(page).not.toHaveURL(/person_id|person_role|unwatched|video_codec/);
});

test('profile preference load failures offer recovery', async ({ page }) => {
  await page.route('**/api/me/prefs', r => r.fulfill({ status: 503, json: { error: 'Profile preferences unavailable' } }));
  await page.goto('/settings');
  await expect(page.getByText('Profile preferences unavailable', { exact: true })).toBeVisible();
  await page.unroute('**/api/me/prefs');
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByRole('switch', { name: 'Hide spoilers', exact: true })).toBeVisible();
  // Show missing episodes is one setting for every show, saved on the profile.
  const missing = page.getByRole('switch', { name: 'Show missing episodes', exact: true });
  await expect(missing).not.toBeChecked();
  await missing.click();
  await expect(missing).toBeChecked();
  expect((await (await page.request.get('/api/me/prefs')).json()).show_missing_episodes).toBe(true);
});

test('queue editing preserves current playback, browsing and saved episode progress', async ({ page }, info) => {
  const shows = await seed(page, true); await seed(page);
  let starts = 0;
  page.on('request', r => { if (r.method() === 'POST' && r.url().endsWith('/playback')) starts++; });
  await page.goto('/series/' + shows.item.id);
  await page.getByRole('button', { name: /^Play S\d+ · E\d+$/ }).click();
  const video = page.locator('video');
  const cdp = await page.context().newCDPSession(page); await cdp.send('Emulation.setFocusEmulationEnabled', { enabled: true });
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(0);
  for (let i = 0; i < 7; i++) await page.keyboard.press('l');
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(65);
  // Space pauses only when the player, not some button, has focus (a focused button takes Space
  // as a click, as browsers do); focus it so the keypress can't start another episode instead.
  await page.getByRole('region', { name: 'Media player', exact: true }).focus();
  await page.keyboard.press('Space');
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.paused)).toBe(true);
  await page.getByRole('button', { name: 'Minimize player', exact: true }).click();
  await page.locator('aside').getByRole('link', { name: 'Audit Movies', exact: true }).click();
  await page.getByRole('link', { name: /Arc Movie/ }).first().click({ position: { x: 12, y: 60 } }); // beside the poster's hover buttons
  await page.getByRole('main').getByRole('button', { name: 'More', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Play next', exact: true }).click();
  await page.getByRole('button', { name: 'Playback queue', exact: true }).click();
  const queue = page.getByRole('dialog', { name: 'Playback queue', exact: true });
  await expect(queue.locator('ol li').nth(0)).toContainText('First Chapter');
  await expect(queue.locator('ol li').nth(1)).toContainText('Arc Movie');
  await expect(queue.locator('ol li').nth(2)).toContainText('Second Chapter');
  await queue.getByRole('button', { name: 'Move Arc Movie down', exact: true }).click();
  await expect(queue.locator('ol li').nth(1)).toContainText('Second Chapter');
  await queue.getByRole('button', { name: 'Remove Second Chapter from queue', exact: true }).click();
  await expect(queue.locator('ol li')).toHaveCount(2);
  // The server keeps the same queue for this device, so a reload can pick it up.
  await expect.poll(() => serverQueue(page)).toEqual(['First Chapter', 'Arc Movie']);
  expect(starts).toBe(1);
  expect(await video.evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(65);
  await page.screenshot({ path: info.outputPath('editable-queue.png') });
  await queue.getByRole('button', { name: 'Clear upcoming', exact: true }).click();
  await expect(queue.locator('ol li')).toHaveCount(1);
  await expect.poll(() => serverQueue(page)).toEqual(['First Chapter']);
  await queue.getByRole('button', { name: 'Close', exact: true }).click();
  await page.getByRole('button', { name: 'Close player', exact: true }).click();
  // Closing the player is a deliberate stop: the server's queue goes too.
  await expect.poll(async () => (await page.request.get('/api/play-queues/current')).status()).toBe(404);
  await expect.poll(async () => (await (await page.request.get('/api/home/continue')).json()).some((i: Item) => i.title === 'First Chapter' && (i.user_data?.position_ms || 0) > 65_000)).toBe(true);
});

test('the server name is set in General settings and is what apps are told', async ({ page }) => {
  await page.goto('/settings?section=general');
  const name = page.getByRole('textbox', { name: 'Server name', exact: true });
  await name.fill('  Living Room  ');
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect.poll(async () => (await (await page.request.get('/api/identity')).json()).name).toBe('Living Room');
  await expect(page.getByRole('button', { name: 'Save', exact: true })).toHaveCount(0);
  await page.reload();
  await expect(page.getByRole('textbox', { name: 'Server name', exact: true })).toHaveValue('Living Room');
});

test('a reload offers the queue this device was playing, to resume or dismiss', async ({ page }, info) => {
  const shows = await seed(page, true);
  await page.goto('/series/' + shows.item.id);
  await page.getByRole('button', { name: /^Play S\d+ · E\d+$/ }).click();
  const video = page.locator('video');
  const cdp = await page.context().newCDPSession(page); await cdp.send('Emulation.setFocusEmulationEnabled', { enabled: true });
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(0);
  await expect.poll(() => serverQueue(page)).toEqual(['First Chapter', 'Second Chapter']);
  await page.reload();
  const offer = page.getByRole('region', { name: 'Pick up your queue', exact: true });
  await expect(offer).toContainText('First Chapter and 1 more');
  await page.screenshot({ path: info.outputPath('recovered-queue.png') });
  await offer.getByRole('button', { name: 'Resume', exact: true }).click();
  await expect(offer).toBeHidden();
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(0);
  await page.getByRole('button', { name: 'Playback queue', exact: true }).click();
  const queue = page.getByRole('dialog', { name: 'Playback queue', exact: true });
  await expect(queue.locator('ol li')).toHaveCount(2);
  await expect(queue.locator('ol li').nth(1)).toContainText('Second Chapter');
  // Next moves the server's queue on too.
  await queue.getByRole('button', { name: 'Play Second Chapter now', exact: true }).click();
  // (Autoplay next episode may already have added the following episode after it, on the server too.)
  await expect.poll(async () => (await serverQueue(page))[0]).toBe('Second Chapter');
  // Leaving the player some other way (a reload) and dismissing the offer discards it.
  await page.reload();
  await page.getByRole('region', { name: 'Pick up your queue', exact: true }).getByRole('button', { name: 'Dismiss queue', exact: true }).click();
  await expect.poll(async () => (await page.request.get('/api/play-queues/current')).status()).toBe(404);
  await page.reload();
  await page.waitForLoadState('networkidle');
  await expect(page.getByRole('region', { name: 'Pick up your queue', exact: true })).toHaveCount(0);
});

/** The server's play queue for this browser, from the current title on (titles only). */
async function serverQueue(page: Page) {
  const r = await page.request.get('/api/play-queues/current');
  if (!r.ok()) return [];
  const q: { current_entry_id: string; entries: { id: string; item?: Item }[] } = await r.json();
  return q.entries.slice(Math.max(0, q.entries.findIndex(e => e.id === q.current_entry_id))).map(e => e.item?.title ?? '');
}

test('playback failures can retry without reopening the title', async ({ page }) => {
  const { item } = await seed(page);
  await page.route('**/playback', r => r.fulfill({ status: 503, json: { error: 'Playback temporarily unavailable' } }));
  await page.goto('/movie/' + item.id);
  await page.getByRole('button', { name: 'Play', exact: true }).click();
  await expect(page.getByRole('heading', { name: "Can't play this", exact: true })).toBeVisible();
  await page.unroute('**/playback');
  await page.getByRole('button', { name: 'Retry playback', exact: true }).click();
  await expect.poll(() => page.locator('video').evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(0);
  await page.getByRole('button', { name: 'Close player', exact: true }).click();
});

test('season labels follow the library with a per-show override', async ({ page }) => {
  const { lib, item } = await seed(page, true);
  await page.goto('/series/' + item.id);
  await openOrdering(page);
  await chooseDisplay(page, /Story Order/);
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByRole('dialog', { name: 'Edit', exact: true })).toBeHidden();
  await expect(page.getByRole('button', { name: 'Season 1', exact: true })).toBeVisible();

  await page.goto('/libraries');
  const row = page.getByRole('region', { name: lib.name, exact: true });
  await row.getByRole('button', { name: 'Edit', exact: true }).click();
  const editor = page.getByRole('dialog');
  await editor.getByRole('tab', { name: 'Advanced', exact: true }).click();
  const libraryLabels = editor.getByRole('combobox', { name: 'Season labels', exact: true });
  await expect(libraryLabels).toHaveValue('number');
  await libraryLabels.selectOption('name');
  await editor.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(editor).toBeHidden();

  await page.goto('/series/' + item.id);
  await expect(page.getByRole('button', { name: 'Story Season', exact: true })).toBeVisible();
  await openOrdering(page);
  const showLabels = page.getByRole('combobox', { name: 'Season labels', exact: true });
  await expect(showLabels).toHaveValue('');
  await showLabels.selectOption('number');
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByRole('dialog', { name: 'Edit', exact: true })).toBeHidden();
  await expect(page.getByRole('button', { name: 'Season 1', exact: true })).toBeVisible();
  await openOrdering(page);
  await expect(showLabels).toHaveValue('number');
  await showLabels.selectOption('');
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Story Season', exact: true })).toBeVisible();
});

test('comparison stages selections until Save, preserves Retry and remains owner-only', async ({ page }) => {
  const { item } = await seed(page, true);
  await page.goto('/series/' + item.id);
  await openOrdering(page);

  await page.getByRole('button', { name: 'Compare orders…',exact:true }).click();
  const dialog = page.getByRole('dialog', { name: 'Compare Episode Orders', exact: true });
  const before = await (await page.request.get(`/api/series/${item.id}/orderings`)).json();
  await dialog.getByRole('button', { name: 'Use Story Order', exact: true }).click();
  await expect(dialog).not.toBeVisible();
  await expect(page.getByRole('combobox',{name:'Display order',exact:true}).locator('option:checked')).toContainText('Story Order');
  expect((await (await page.request.get(`/api/series/${item.id}/orderings`)).json()).display_ordering_id).toBe(before.display_ordering_id);
  await page.getByRole('button',{name:'Cancel',exact:true}).click();
  await openOrdering(page);
  await expect(page.getByRole('combobox',{name:'Display order',exact:true}).locator('option:checked')).toContainText('TMDB · Aired Order');
  await page.getByRole('button', { name: 'Compare orders…',exact:true }).click();
  await dialog.getByRole('button', { name: 'Use Story Order', exact: true }).click();
  await page.route('**/display-ordering', r => r.fulfill({ status: 503, json: { error: 'Comparison save unavailable' } }));
  await page.getByRole('button',{name:'Save',exact:true}).click();
  await expect(page.getByRole('dialog',{name:'Edit',exact:true}).getByRole('alert')).toContainText('Comparison save unavailable');
  await expect(page.getByRole('combobox',{name:'Display order',exact:true}).locator('option:checked')).toContainText('Story Order');
  await page.unroute('**/display-ordering');
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  const inherited=(await (await page.request.get(`/api/series/${item.id}/orderings`)).json()).display_ordering_id;
  const viewer = await (await page.request.post('/api/profiles', { data: { name: 'Order viewer', color: 'blue' } })).json();
  expect((await page.request.post('/api/auth/profile', { data: { id: viewer.id } })).ok()).toBe(true);
  await page.reload();
  // A profile sees the owner's order and has no way to change it.
  await page.getByRole('button',{name:'More',exact:true}).click();
  await page.getByRole('menu').waitFor();
  await expect(page.getByRole('menuitem',{name:'Edit…',exact:true})).toHaveCount(0);
  await expect(page.getByRole('menuitem',{name:'Episode ordering…',exact:true})).toHaveCount(0);
  await page.keyboard.press('Escape');
  const orders = (await (await page.request.get(`/api/series/${item.id}/orderings`)).json()).orderings as { id: number }[];
  expect((await page.request.put(`/api/series/${item.id}/display-ordering`, { data: { ordering_id: orders.find(o => o.id !== inherited)!.id } })).status()).toBe(403);
  expect((await page.request.put(`/api/series/${item.id}/display-ordering`, { data: { ordering_id: null } })).status()).toBe(403);
  expect((await (await page.request.get(`/api/series/${item.id}/orderings`)).json()).display_ordering_id).toBe(inherited);
});

test('paused IMDb ratings hide stale scores and sort choices while provider and personal scores remain', async ({ page }) => {
  const { item, lib } = await seed(page);
  let critics = false;
  await page.route('**/api/items/' + item.id, async route => {
    const response = await route.fetch();
    await route.fulfill({ response, json: { ...await response.json(), rating: 7.4,
      imdb_rating: 9.9, imdb_votes: 12345,
      rotten_tomatoes: critics ? 83 : null, metacritic: critics ? 71 : null } });
  });
  await page.goto('/movie/' + item.id);
  await expect(page.getByRole('heading', { name: item.title, exact: true })).toBeVisible();
  await expect(page.getByText('IMDb', { exact: true })).toHaveCount(0);
  await expect(page.getByText('★ 7.4', { exact: true })).toBeVisible();
  await expect(page.getByRole('radio', { name: '5 stars', exact: true })).toBeVisible();
  critics = true;
  await page.reload();
  await expect(page.getByText('83%', { exact: true })).toBeVisible();
  await expect(page.locator('[title="Metacritic"]')).toContainText('71');
  await expect(page.getByText('IMDb', { exact: true })).toHaveCount(0);
  // Saved pre-pause IMDb browse URLs recover to an available sort.
  await page.goto('/library/' + lib.id + '?sort=imdb');
  await page.getByRole('button', { name: 'Sort by Title', exact: true }).click();
  await expect(page.getByRole('menuitemradio', { name: /IMDb/ })).toHaveCount(0);
  await expect(page.getByRole('menuitemradio', { name: 'Title', exact: true })).toBeVisible();
  await page.keyboard.press('Escape');
  await page.goto('/settings?section=library');
  await expect(page.getByText('Rotten Tomatoes and Metacritic', { exact: true })).toBeVisible();
  await expect(page.getByText('IMDb', { exact: true })).toHaveCount(0);
});

test('ratings, watchlist and metadata refresh failures explain and recover', async ({ page }) => {
  const { item } = await seed(page);
  await page.goto('/movie/' + item.id);
  for (const [suffix, action] of [['rating', '5 stars'], ['watchlist', 'Add to Watchlist'], ['refresh', 'Refresh metadata']]) {
    await page.route('**/' + suffix, r => r.fulfill({ status: 503, json: { error: suffix + ' unavailable' } }));
    if (suffix === 'refresh') await page.getByRole('button', { name: 'More', exact: true }).click();
    await page.getByRole(suffix === 'rating' ? 'radio' : suffix === 'refresh' ? 'menuitem' : 'button', { name: action, exact: true }).click();
    await expect(page.getByText(suffix + ' unavailable', { exact: true })).toBeVisible();
    await page.unroute('**/' + suffix);
    await page.getByRole('button', { name: 'Retry', exact: true }).click();
    await expect(page.getByText(suffix + ' unavailable', { exact: true })).toHaveCount(0);
  }
  await page.reload();
  await expect(page.getByRole('button', { name: 'On your watchlist', exact: true })).toBeVisible();
});

test('playlist keyboard ordering and failed writes retain a recoverable list', async ({ page }) => {
  const { items } = await seed(page);
  const p = await (await page.request.post('/api/playlists', { data: { title: 'Keyboard playlist', item_ids: items.map(i => i.id) } })).json();
  await page.goto('/playlist/' + p.id);
  const move = page.getByRole('button', { name: 'Move ' + items[1].title + ' up', exact: true });
  await move.focus(); await page.keyboard.press('Enter');
  await expect.poll(async () => (await (await page.request.get('/api/playlists/' + p.id)).json()).entries[0].item.id).toBe(items[1].id);
  await page.route('**/api/playlists/*/order', r => r.fulfill({ status: 503, json: { error: 'Playlist order unavailable' } }));
  await page.getByRole('button', { name: 'Move ' + items[0].title + ' up', exact: true }).click();
  await expect(page.getByText('Playlist order unavailable', { exact: true })).toBeVisible();
  await page.unroute('**/api/playlists/*/order');
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect.poll(async () => (await (await page.request.get('/api/playlists/' + p.id)).json()).entries[0].item.id).toBe(items[0].id);
});

for (const shows of [false, true]) {
  test(`${shows ? 'series' : 'movie'} library remembers filters and sorts after navigation, Back, reload and clear`, async ({ page }) => {
    const { lib, item } = await seed(page, shows);
    await page.goto('/library/' + lib.id);
    await page.getByRole('button', { name: 'Filters', exact: true }).click();
    await page.getByRole('switch', { name: "Only titles I haven't watched", exact: true }).click();
    await expect(page.getByRole('switch', { name: "Only titles I haven't watched", exact: true })).toBeChecked();
    await page.keyboard.press('Escape');
    await page.getByRole('button', { name: 'Order: A–Z', exact: true }).click();
    await page.getByRole('menuitemradio', { name: 'Z–A', exact: true }).click();
    await page.locator('main').getByRole('link', { name: new RegExp(item.title) }).first().locator('p').filter({ hasText: item.title }).first().click();
    await page.goBack();
    await expect(page.getByRole('button', { name: 'Remove filter: Unwatched', exact: true })).toBeVisible();
    await page.reload();
    await expect(page.getByRole('button', { name: 'Order: Z–A', exact: true })).toBeVisible();
    await page.locator('aside').getByRole('link', { name: 'Home', exact: true }).click();
    await page.locator('aside').getByRole('link', { name: lib.name, exact: true }).click();
    await expect(page.getByRole('button', { name: 'Remove filter: Unwatched', exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Order: Z–A', exact: true })).toBeVisible();
    await page.locator('aside').getByRole('link', { name: lib.name, exact: true }).click();
    await expect(page.getByRole('button', { name: 'Remove filter: Unwatched', exact: true })).toBeVisible();
    await page.getByRole('button', { name: 'Filters, 1 on', exact: true }).click();
    await page.getByRole('button', { name: 'Clear filters', exact: true }).click();
    await page.keyboard.press('Escape');
    await page.locator('aside').getByRole('link', { name: 'Home', exact: true }).click();
    await page.locator('aside').getByRole('link', { name: lib.name, exact: true }).click();
    await expect(page.getByRole('button', { name: 'Filters', exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Remove filter: Unwatched', exact: true })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Order: Z–A', exact: true })).toBeVisible();
  });
}

test('remembered filters are isolated by library and actual profile switching', async ({ page }) => {
  const { lib: movies } = await seed(page);
  const { lib: shows } = await seed(page, true);
  const viewer = await (await page.request.post('/api/profiles', { data: { name: 'Filter viewer', color: 'blue' } })).json();
  await page.goto('/library/' + movies.id);
  await page.getByRole('button', { name: 'Filters', exact: true }).click();
  await page.getByRole('switch', { name: "Only titles I haven't watched", exact: true }).click();
    await expect(page.getByRole('switch', { name: "Only titles I haven't watched", exact: true })).toBeChecked();
  await page.keyboard.press('Escape');
  await page.locator('aside').getByRole('link', { name: shows.name, exact: true }).click();
  await expect(page.getByRole('button', { name: 'Filters', exact: true })).toBeVisible();
  await page.locator('aside').getByRole('link', { name: movies.name, exact: true }).click();
  await expect(page.getByRole('button', { name: 'Remove filter: Unwatched', exact: true })).toBeVisible();
  await page.locator('aside').getByRole('button', { name: 'Admin Admin', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Switch Profile', exact: true }).click();
  await page.getByRole('button', { name: 'Filter viewer', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Filters', exact: true })).toBeVisible();
  await expect(page).not.toHaveURL(/unwatched/);
  await page.locator('aside').getByRole('button', { name: 'Filter viewer', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Switch Profile', exact: true }).click();
  await page.getByRole('button', { name: 'Admin', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Remove filter: Unwatched', exact: true })).toBeVisible();
  expect(await page.evaluate(({ id, library }) => localStorage.getItem(`arc-browse:${id}:${library}`), { id: viewer.id, library: movies.id })).toBeNull();
});

test('explicit library links override saved filters and empty results clear durably', async ({ page }) => {
  const { lib } = await seed(page);
  await page.goto('/library/' + lib.id);
  await page.getByRole('button', { name: 'Filters', exact: true }).click();
  await page.getByRole('switch', { name: "Only titles I haven't watched", exact: true }).click();
    await expect(page.getByRole('switch', { name: "Only titles I haven't watched", exact: true })).toBeChecked();
  await page.keyboard.press('Escape');
  await page.goto(`/library/${lib.id}?genre=NoSuchGenre&view=titles&letter=a`);
  await expect(page.getByRole('button', { name: 'Remove filter: NoSuchGenre', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Remove filter: Unwatched', exact: true })).toHaveCount(0);
  await expect(page).not.toHaveURL(/letter=/);
  await page.getByRole('button', { name: 'Sort by Title', exact: true }).click();
  await page.getByRole('menuitemradio', { name: 'Year', exact: true }).click();
  await page.locator('aside').getByRole('link', { name: 'Home', exact: true }).click();
  await page.locator('aside').getByRole('link', { name: lib.name, exact: true }).click();
  await expect(page.getByRole('button', { name: 'Clear filters', exact: true })).toBeVisible();
  await expect(page).not.toHaveURL(/view=|letter=/);
  await page.getByRole('button', { name: 'Clear filters', exact: true }).click();
  await page.locator('aside').getByRole('link', { name: 'Home', exact: true }).click();
  await page.locator('aside').getByRole('link', { name: lib.name, exact: true }).click();
  await expect(page.getByRole('button', { name: 'Filters', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Sort by Year', exact: true })).toBeVisible();
});

test('invalid library storage is ignored and unavailable storage leaves URL controls usable', async ({ page }) => {
  const { lib } = await seed(page);
  const profile = (await (await page.request.get('/api/auth/status')).json()).profile.id;
  await page.evaluate(({ profile, lib }) => localStorage.setItem(`arc-browse:${profile}:${lib}`, 'sort=not-a-sort&resolution=invalid&unwatched=no&view=recommended&letter=z&offset=500'), { profile, lib: lib.id });
  await page.goto('/library/' + lib.id);
  await expect(page.getByRole('button', { name: 'Sort by Title', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Filters', exact: true })).toBeVisible();
  await expect(page).not.toHaveURL(/resolution=|unwatched=|view=|letter=|offset=/);
  await page.addInitScript(() => {
    const get = Storage.prototype.getItem, set = Storage.prototype.setItem;
    Storage.prototype.getItem = function(key: string) { if (key.startsWith('arc-browse:')) throw new DOMException('Storage disabled', 'SecurityError'); return get.call(this, key); };
    Storage.prototype.setItem = function(key: string, value: string) { if (key.startsWith('arc-browse:')) throw new DOMException('Storage disabled', 'SecurityError'); set.call(this, key, value); };
  });
  await page.goto('/library/' + lib.id);
  await page.getByRole('button', { name: 'Filters', exact: true }).click();
  await page.getByRole('switch', { name: "Only titles I haven't watched", exact: true }).click();
    await expect(page.getByRole('switch', { name: "Only titles I haven't watched", exact: true })).toBeChecked();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('button', { name: 'Remove filter: Unwatched', exact: true })).toBeVisible();
  await page.reload();
  await expect(page.getByRole('button', { name: 'Remove filter: Unwatched', exact: true })).toBeVisible();
});

test('Recently Added heading opens explicit added sorting instead of remembered title order', async ({ page }) => {
  const { lib } = await seed(page);
  await page.goto('/');
  await page.getByRole('link', { name: 'Recently Added in ' + lib.name, exact: true }).click();
  await expect(page).toHaveURL(new RegExp('/library/' + lib.id + '\\?sort=added'));
  await expect(page.getByRole('button', { name: 'Sort by Recently Added', exact: true })).toBeVisible();
});

test('browse sort direction is remembered per profile on this device', async ({ page }) => {
  const { lib } = await seed(page);
  await page.goto('/library/' + lib.id);
  const original = (await (await page.request.get('/api/items?library_id=' + lib.id + '&sort=title')).json()).items.map((i: Item) => i.id);
  const reversed = (await (await page.request.get('/api/items?library_id=' + lib.id + '&sort=title&reverse=1')).json()).items.map((i: Item) => i.id);
  expect(reversed).toEqual([...original].reverse());
  await page.getByRole('button', { name: 'Order: A–Z', exact: true }).click();
  await page.getByRole('menuitemradio', { name: 'Z–A', exact: true }).click();
  await expect(page).toHaveURL(/reverse=1/);
  await page.goto('/library/' + lib.id);
  await expect(page.getByRole('button', { name: 'Order: Z–A', exact: true })).toBeVisible();
  const viewer = await (await page.request.post('/api/profiles', { data: { name: 'Browse viewer', color: 'blue' } })).json();
  await page.request.post('/api/auth/profile', { data: { id: viewer.id } });
  await page.goto('/library/' + lib.id);
  await expect(page.getByRole('button', { name: 'Order: A–Z', exact: true })).toBeVisible();
});

test('switching profiles stops compact playback after saving the original profile progress', async ({ page }) => {
  const { item } = await seed(page);
  await page.request.post('/api/profiles', { data: { name: 'Compact viewer', color: 'blue' } });
  await page.goto('/movie/' + item.id);
  await page.getByRole('button', { name: 'Play', exact: true }).click();
  await expect.poll(() => page.locator('video').evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(0);
  for (let i = 0; i < 7; i++) await page.keyboard.press('l');
  await expect.poll(() => page.locator('video').evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(65);
  await page.getByRole('button', { name: 'Minimize player', exact: true }).click();
  await page.locator('aside').getByRole('button', { name: /Admin/ }).click();
  await page.getByRole('menuitem', { name: 'Switch Profile', exact: true }).click();
  await page.getByRole('button', { name: 'Compact viewer', exact: true }).click();
  await expect(page.locator('video')).toHaveCount(0);
  expect((await (await page.request.get('/api/home/continue')).json()).length).toBe(0);
  const profiles = (await (await page.request.get('/api/auth/status')).json()).profiles;
  const admin = profiles.find((p: { is_admin: boolean }) => p.is_admin);
  await page.request.post('/api/auth/profile', { data: { id: admin.id, password } });
  expect((await (await page.request.get('/api/home/continue')).json()).some((i: Item) => i.id === item.id && (i.user_data?.position_ms || 0) > 65_000)).toBe(true);
});

test('compact playback carries on to the next episode after its countdown', async ({ page }) => {
  const { item } = await seed(page, true);
  await page.goto('/series/' + item.id);
  await page.getByRole('button', { name: /^Play S\d+ · E\d+$/ }).click();
  const video = page.locator('video');
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(0);
  await page.getByRole('button', { name: 'Minimize player', exact: true }).click();
  await expect(page.getByRole('button', { name: /Next: Second Chapter/ })).toBeVisible();
  await video.evaluate((v: HTMLVideoElement) => { v.currentTime = v.duration - 0.3; });
  await expect(page.getByText('Up next in 10', { exact: true })).toBeVisible();
  await expect(page.getByRole('region', { name: 'Media player' }).getByText('Second Chapter', { exact: true })).toBeVisible({ timeout: 20000 });
  await expect(video).toHaveCount(1); // Wait for the previous player's exit animation.
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(0);
  await page.getByRole('button', { name: 'Close player', exact: true }).click();
});

test('maintenance Run explains accepted, queued, waiting, running and finished states', async ({ page }) => {
  let task = { kind: 'database', title: 'Optimize database', enabled: true, requested: false,
    state: 'scheduled', detail: 'Waiting for maintenance', pending: 0, completed: 0, failed: 0,
    last_run: undefined as undefined | { started_at: number; finished_at: number; last_success_at: number; detail: string } };
  await page.route('**/api/maintenance', route => route.fulfill({ json: [task] }));
  let requests = 0;
  await page.route('**/api/maintenance/database/run', async route => {
    requests++;
    task = { ...task, requested: true, state: 'queued', detail: 'Run requested; waiting for an available background task slot' };
    await route.fulfill({ status: 202, json: { queued: true } });
  });
  await page.goto('/settings?section=maintenance');
  const run = page.getByRole('button', { name: 'Run Optimize database now', exact: true });
  await run.click();
  await expect(page.getByText('Queued', { exact: true })).toBeVisible();
  await expect(run).toBeDisabled();
  expect(requests).toBe(1);
  task = { ...task, state: 'paused', detail: 'Waiting for playback or scanning' };
  await expect(page.getByText('Waiting for playback or scanning', { exact: false })).toBeVisible();
  task = { ...task, state: 'running', detail: 'Checking database integrity' };
  await expect(page.getByText('Running', { exact: true })).toBeVisible();
  task = { ...task, requested: false, state: 'completed', detail: 'Next automatic run is weekly',
    last_run: { started_at: 100, finished_at: 110, last_success_at: 110, detail: 'Compaction not needed' } };
  await expect(page.getByText(/Last result.*Compaction not needed/)).toBeVisible();
  await expect(run).toBeEnabled();
});

test('diagnostics can be enabled, downloaded and retained across restart with admin protection', async ({ page }, info) => {
  await page.goto('/settings?section=maintenance');
  const toggle = page.getByRole('switch', { name: 'Detailed logging', exact: true });
  await expect(toggle).not.toBeChecked();
  await page.route('**/api/diagnostics', r => r.request().method() === 'PATCH' ? r.fulfill({ status: 503, json: { error: 'Diagnostics unavailable' } }) : r.continue());
  await toggle.click();
  await expect(page.getByText('Diagnostics unavailable', { exact: true })).toBeVisible();
  await page.unroute('**/api/diagnostics');
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByText('Record extra detail for troubleshooting. Turns off automatically after 2 hours.', { exact: true })).toBeVisible();
  const diagnosticStatus = await (await page.request.get('/api/diagnostics')).json();
  const remaining = new Date(diagnosticStatus.until).getTime() - Date.now();
  expect(remaining).toBeGreaterThan(119 * 60 * 1000);
  expect(remaining).toBeLessThanOrEqual(120 * 60 * 1000);
  await expect(page.getByText(/^Detailed logging ends at /)).toBeVisible();
  await expect(toggle).toBeChecked();
  await page.reload(); await expect(toggle).toBeChecked();
  await page.route('**/api/diagnostics/download', r => r.fulfill({ status: 503 }));
  await page.getByRole('button', { name: 'Download diagnostics', exact: true }).click();
  await expect(page.getByText('Could not download diagnostics. Please try again.', { exact: true })).toBeVisible();
  await page.unroute('**/api/diagnostics/download');
  const download = page.waitForEvent('download');
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  expect((await download).suggestedFilename()).toBe('arc-diagnostics.zip');
  expect(await (await download).failure()).toBeNull();
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  await page.screenshot({ path: info.outputPath('diagnostics-390.png'), fullPage: true });
  await page.request.post('http://127.0.0.1:18200/restart');
  await page.reload(); await expect(toggle).not.toBeChecked();
  expect((await (await page.request.get('/api/logs')).json()).some((l: { message: string }) => l.message === 'detailed logging changed')).toBe(true);
  const viewer = await (await page.request.post('/api/profiles', { data: { name: 'Diagnostics viewer', color: 'blue' } })).json();
  await page.request.post('/api/auth/profile', { data: { id: viewer.id } });
  for (const method of ['GET', 'PATCH']) expect((await page.request.fetch('/api/diagnostics', { method, ...(method === 'PATCH' ? { data: { detailed: true } } : {}) })).status()).toBe(403);
  expect((await page.request.get('/api/diagnostics/download')).status()).toBe(403);
});

test('poster-only saves skip unchanged metadata and show artwork progress', async ({ page }, info) => {
  const { item } = await seed(page, true);
  let edits = 0, refreshes = 0;
  page.on('request', r => { if (r.method() === 'PATCH' && r.url().endsWith('/api/items/' + item.id)) edits++; if (r.url().endsWith('/refresh')) refreshes++; });
  const image = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a3ioAAAAASUVORK5CYII=', 'base64');
  await page.route('**/api/items/*/images/*', r => r.fulfill({ contentType: 'image/png', body: image }));
  let release!: () => void;
  const saving = new Promise<void>(r => { release = r; });
  await page.route('**/api/items/' + item.id + '/artwork', async r => {
    if (r.request().method() === 'PUT') { await saving; return r.continue(); }
    return r.fulfill({ json: { posters: [{ path: '/fixture-poster.jpg', thumb: 'data:image/png;base64,' + image.toString('base64'), width: 600, height: 900 }], backdrops: [], logos: [], current: {}, chosen: {}, local: {} } });
  });
  await page.goto('/series/' + item.id);
  await page.getByRole('button', { name: 'More', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Edit…', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'Edit', exact: true });
  await dialog.getByRole('tab', { name: 'Poster', exact: true }).click();
  await dialog.getByRole('button', { name: '600×900', exact: true }).click();
  await dialog.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(dialog.getByRole('status')).toHaveText('Saving poster…');
  await page.screenshot({ path: info.outputPath('poster-saving.png') });
  expect(edits).toBe(0); release();
  await expect(dialog).not.toBeVisible();
  expect(refreshes).toBe(0);
  const saved = await (await page.request.get('/api/items/' + item.id)).json();
  expect(saved.locked_fields).toContain('poster');
});

test.beforeEach(async ({ page }) => {
  const r = await fetch('http://127.0.0.1:18200/reset', { method: 'POST' });
  if (!r.ok) throw new Error(await r.text());
  media = await r.json();
  await page.context().route('**/*', route => { const u = new URL(route.request().url()); return u.hostname === '127.0.0.1' || u.protocol === 'data:' ? route.continue() : route.abort(); });
  await setup(page);
});
async function seed(page: Page, shows = false) {
  const r = await page.request.post('/api/libraries', { data: { name: shows ? 'Audit TV' : 'Audit Movies', kind: shows ? 'shows' : 'movies', paths: [shows ? media.shows : media.movies] } });
  expect(r.ok()).toBe(true); const lib: Library = await r.json();
  await expect.poll(async () => (await (await page.request.get('/api/libraries/' + lib.id + '/scan')).json()).running, { timeout: 30000 }).toBe(false);
  const items: Item[] = (await (await page.request.get('/api/items?library_id=' + lib.id)).json()).items;
  return { lib, item: items.find((i: Item) => i.title === (shows ? 'Arc Show' : 'Arc Movie'))!, items };
}
async function setup(page: Page) {
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Choose a database', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Continue', exact: true }).click();
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

test('source labels keep HDR and profile-based audio with their own movie version', async ({ page }, info) => {
  const { item } = await seed(page);
  const original = await (await page.request.get('/api/items/' + item.id)).json();
  const base = original.files[0];
  const file = (id: number, edition: string, plus: boolean, dv: boolean, audioProfile: string) => ({ ...base, id, edition, bitrate: 12500000, streams: [
    { index: 0, kind: 'video', codec: 'hevc', width: 3840, height: 2160, is_default: true, is_forced: false, color_transfer: 'smpte2084', dolby_vision: dv, hdr10_plus: plus, hdr_probe_status: 'sampled', aspect_ratio: 2.39, bitrate: 10000000, frame_rate: 23.976, bit_depth: 10, dolby_vision_profile: 8, color_primaries: 'bt2020' },
    { index: 1, kind: 'audio', codec: 'truehd', profile: audioProfile, channels: 8, language: 'eng', is_default: true, is_forced: false },
  ] });
  const files = [file(101, 'Cinema', true, true, 'Dolby TrueHD + Dolby Atmos'), file(102, 'Standard', false, false, '')];
  await page.route('**/api/items/' + item.id, r => r.fulfill({ json: { ...original, files } }));
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto('/movie/' + item.id);
    const hero = page.locator('main').getByRole('radiogroup', { name: 'Edition' }).locator('..');
    await expect(hero.getByText('Dolby Vision', { exact: true })).toBeVisible();
    await expect(hero.getByText('HDR10+', { exact: true })).toBeVisible();
    await expect(hero.getByText('TrueHD Atmos 7.1', { exact: true })).toBeVisible();
    await page.getByRole('radio', { name: 'Standard', exact: true }).click();
    await expect(hero.getByText('HDR10', { exact: true })).toBeVisible();
    await expect(hero.getByText('Dolby TrueHD 7.1', { exact: true })).toBeVisible();
    await expect(hero.getByText('Dolby Vision', { exact: true })).toHaveCount(0);
    await expect(hero.getByText('TrueHD Atmos 7.1', { exact: true })).toHaveCount(0);
    // Media Info is in the title's More menu, as in Plex; every value is labelled.
    await page.getByRole('button', { name: 'More', exact: true }).click();
    await page.getByRole('menuitem', { name: 'Media Info…', exact: true }).click();
    const dialog = page.getByRole('dialog', { name: 'Media Info', exact: true });
    await expect(dialog.getByText('3840×2160 · 4K', { exact: true }).first()).toBeVisible();
    await expect(dialog.getByText('2.39:1', { exact: true }).first()).toBeVisible();
    await expect(dialog.getByText('10.0 Mb/s', { exact: true }).first()).toBeVisible();
    await expect(dialog.getByText('23.976 fps', { exact: true }).first()).toBeVisible();
    await expect(dialog.getByText('10-bit', { exact: true }).first()).toBeVisible();
    await expect(dialog.getByText('Dolby Vision profile 8 · HDR10+', { exact: true })).toBeVisible();
    await expect(dialog.getByText('HDR10', { exact: true })).toBeVisible();
    for (const label of ['Codec', 'Resolution', 'Aspect ratio', 'Frame rate', 'Bit depth', 'HDR', 'Bitrate', 'Container', 'Size', 'Overall bitrate'])
      await expect(dialog.getByRole('term').filter({ hasText: new RegExp(`^${label}$`) }).first()).toBeVisible();
    await expect(dialog.getByText('—', { exact: true })).toHaveCount(0);
    await expect(dialog.getByText(/HDR inspection/)).toHaveCount(0);
    // The raw values wait under Advanced, worded plainly.
    await expect(dialog.getByText('Checked from a sample of frames').first()).toBeHidden();
    await dialog.getByText('Advanced', { exact: true }).first().click();
    await expect(dialog.getByText('Checked from a sample of frames').first()).toBeVisible();
    await expect(dialog.getByText('bt2020', { exact: true }).first()).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: info.outputPath(`source-formats-${width}.png`), fullPage: true });
    await page.keyboard.press('Escape');
  }
  files[1].edition = 'Cinema';
  await page.goto('/movie/' + item.id);
  const hero = page.locator('main h1').locator('..');
  await expect(hero.getByText('2 versions', { exact: true })).toBeVisible();
  await expect(hero.getByText('TrueHD Atmos 7.1', { exact: true })).toHaveCount(0);
});

test('series file details show episode source tracks without claiming series-wide HDR', async ({ page }) => {
  const { item } = await seed(page, true);
  await page.goto('/series/' + item.id);
  const button = page.getByRole('main').getByRole('button', { name: /^More for / }).first();
  await expect(button).toBeVisible();
  // Intercept only the on-demand item read; the series and episode list remain real.
  await page.route('**/api/items/*', async r => {
    if (r.request().method() !== 'GET') return r.continue();
    const response = await r.fetch(); const episode = await response.json();
    if (episode.kind !== 'episode') return r.fulfill({ response });
    return r.fulfill({ json: { ...episode, files: episode.files.map((f: object) => ({ ...f, streams: [
      { index: 0, kind: 'video', codec: 'hevc', width: 1920, height: 1080, color_transfer: 'arib-std-b67' },
      { index: 1, kind: 'audio', codec: 'eac3', profile: 'Dolby Digital Plus + Dolby Atmos', channels: 6 },
      { index: 2, kind: 'audio', codec: 'dts', profile: 'DTS-HD MA + DTS:X', channels: 8 },
      { index: 3, kind: 'audio', codec: 'truehd', channels: 8, title: 'Atmos DV HDR10+ (misleading track title)' },
    ] })) } });
  });
  await button.click();
  await page.getByRole('menuitem', { name: 'Media Info…', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'Media Info', exact: true });
  await expect(dialog.getByText('1920×1080 · 1080p', { exact: true })).toBeVisible();
  await expect(dialog.getByText('HEVC', { exact: true })).toBeVisible();
  await expect(dialog.getByText('HLG', { exact: true })).toBeVisible();
  const audio = dialog.getByRole('row');
  await expect(audio.filter({ hasText: 'Dolby Digital+ Atmos' })).toContainText('5.1');
  await expect(audio.filter({ hasText: 'DTS-HD MA · DTS:X' })).toContainText('7.1');
  await expect(audio.filter({ hasText: 'Dolby TrueHD' })).toContainText('7.1');
  await expect(dialog.getByText('Dolby Vision', { exact: true })).toHaveCount(0);
  await expect(dialog.getByText(/what plays depends on your device/)).toBeVisible();
  await page.keyboard.press('Escape'); await expect(dialog).toHaveCount(0);
});

test('background scheduling exposes modes, task controls and recoverable Run now', async ({ page }, info) => {
  await seed(page);
  await page.goto('/settings?section=library');
  const interval = page.getByRole('combobox', { name: 'Scheduled library scans', exact: true });
  await expect(interval.getByRole('option', { name: 'Every 15 minutes', exact: true })).toHaveCount(1);
  await expect(interval.getByRole('option', { name: 'Every 30 minutes', exact: true })).toHaveCount(1);
  await expect(interval.getByRole('option', { name: 'Every 2 hours', exact: true })).toHaveCount(1);
  await interval.selectOption('15'); await expect(interval).toBeEnabled();
  await page.reload(); await expect(interval).toHaveValue('15');
  expect((await (await page.request.get('/api/settings')).json()).scan_interval_minutes).toBe(15);
  await page.goto('/settings?section=playback');
  await expect(page.getByRole('combobox', { name: 'Intro & credit detection', exact: true })).toHaveCount(0);
  await page.getByRole('link', { name: 'Scheduled tasks', exact: true }).click();
  await expect(page).toHaveURL(/section=maintenance/);
  const detection = page.getByRole('combobox', { name: 'Intro & credit detection', exact: true });
  await detection.selectOption('off'); await expect(detection).toBeEnabled();
  await page.reload(); await expect(detection).toHaveValue('off');
  const preview = page.getByRole('combobox', { name: 'Scrubbing previews', exact: true });
  await preview.selectOption('scheduled'); await expect(preview).toBeEnabled();
  const analysis = page.getByRole('switch', { name: 'Media analysis', exact: true });
  await analysis.click(); await expect(analysis).not.toBeChecked();
  const run = page.getByRole('button', { name: 'Run Media analysis now', exact: true });
  await expect(run).toBeDisabled();
  await analysis.click(); await expect(analysis).toBeChecked(); await expect(run).toBeEnabled();
  await page.route('**/api/maintenance/analysis/run', r => r.fulfill({ status: 503, json: { error: 'Task could not start' } }));
  await run.click(); await expect(page.getByText('Task could not start', { exact: true })).toBeVisible();
  await page.unroute('**/api/maintenance/analysis/run');
  await page.getByRole('alert').getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByText('Task could not start', { exact: true })).toHaveCount(0);
  await page.setViewportSize({ width: 390, height: 900 });
  await page.reload(); await expect(preview).toHaveValue('scheduled');
  await expect(run).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: info.outputPath('scheduled-work-phone.png'), fullPage: true });
});

test('Dashboard Activity shows concurrent titles, actual stages, file details and complete summaries', async ({ page }, info) => {
  const now = new Date().toISOString();
  await page.route('**/api/activity', route => route.fulfill({ json: {
    running: [
      { id: 10, kind: 'scan', title: 'Scanning Movies', detail: 'Checking files and titles · 420 files checked', started_at: now, current: [
        { id: 1, stage: 'Matching movie', subject: 'Dune (2021)', detail: 'Looking up metadata on TMDB' },
        { id: 2, stage: 'Inspecting file', subject: 'The Last Observatory: Beyond the Northern Horizon (2026)', detail: 'The.Last.Observatory.2026.2160p.TrueHD.Atmos.mkv' },
        { id: 3, stage: 'Saving movie details', subject: 'Arrival (2016)', detail: 'Saving metadata and artwork references' },
      ] },
      { id: 11, kind: 'scan', title: 'Scanning Series', detail: 'Checking files and titles · 30 files checked', started_at: now, current: [
        { id: 4, stage: 'Linking episodes', subject: 'Severance', detail: 'Using saved episode metadata', done: 6, total: 10 },
      ] },
      { id: 12, kind: 'analysis', title: 'Analyzing media in Movies', detail: 'Background video, audio and HDR inspection', started_at: now, current: [
        { id: 5, stage: 'Inspecting source formats', subject: 'Blade Runner (1982)', detail: 'Blade.Runner.1982.mkv' },
      ] },
    ],
    recent: [{ id: 9, kind: 'scan', title: 'Scanning Completed Library', started_at: now, finished_at: now,
      detail: '420 files checked · 3 file details read · 416 reused saved details · 1 inspection error · 2 identified · 1 not matched', error: 'Stopped before it finished' }],
  } }));
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto('/dashboard');
    const panel = page.locator('section').filter({ has: page.getByRole('heading', { name: 'Activity', exact: true }) });
    await expect(panel.getByText('Matching movie · Dune (2021)', { exact: true })).toBeVisible();
    await expect(panel.getByText('Inspecting file · The Last Observatory: Beyond the Northern Horizon (2026)', { exact: true })).toBeVisible();
    await expect(panel.getByText('6 of 10', { exact: true })).toBeVisible();
    await expect(panel.getByText('Inspecting source formats · Blade Runner (1982)', { exact: true })).toBeVisible();
    await expect(panel.getByText(/\d+%/, { exact: true })).toHaveCount(0); // Discovery totals are unknown.
    await panel.getByText('Show 1 more active operation', { exact: true }).click();
    await expect(panel.getByText('Saving movie details · Arrival (2016)', { exact: true })).toBeVisible();
    const file = panel.getByText('Inspecting file · The Last Observatory: Beyond the Northern Horizon (2026)', { exact: true }).locator('..');
    await file.getByText('Details', { exact: true }).click();
    await expect(file.getByText('The.Last.Observatory.2026.2160p.TrueHD.Atmos.mkv', { exact: true })).toBeVisible();
    await expect(panel.getByText(/416 reused saved details/)).toBeVisible();
    await expect(panel.getByText('Stopped before it finished', { exact: true })).toBeVisible();
    expect(await panel.evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await panel.getByText('Matching movie · Dune (2021)', { exact: true }).scrollIntoViewIfNeeded();
    await page.screenshot({ path: info.outputPath(`activity-titles-${width}.png`), animations: 'disabled' });
  }
});

test('the Activity menu stays simple: task, current title, progress and recent work', async ({ page }, info) => {
  const now = new Date().toISOString();
  let stopped = 0;
  await page.route('**/api/activity/10/stop', route => { stopped++; return route.fulfill({ status: 202, json: {} }); });
  await page.route('**/api/activity', route => route.fulfill({ json: {
    running: [
      { id: 10, kind: 'scan', title: 'Scanning Movies', detail: 'Checking files and titles · 420 files checked', started_at: now, can_stop: true, current: [
        { id: 1, stage: 'Matching movie', subject: 'Dune (2021)', detail: 'Looking up metadata on TMDB' },
        { id: 2, stage: 'Inspecting file', subject: 'Arrival (2016)', detail: 'Arrival.2016.mkv' },
        { id: 3, stage: 'Checking file', subject: 'Blade Runner (1982).mkv', detail: 'Checking for changes' },
        { id: 4, stage: 'Matching movie', subject: 'Heat (1995)', detail: 'Looking up metadata on TMDB' },
      ] },
      { id: 12, kind: 'refresh', title: 'Refreshing metadata in Movies', detail: 'Refreshing metadata in Movies', started_at: now, done: 3, total: 12 },
      { id: 13, kind: 'scan', title: 'Scanning TV Shows', detail: 'Reading file details · 3 of 40', started_at: now, done: 3, total: 40 },
      // Waiting for its turn (no current item): not listed, as in Plex.
      { id: 14, kind: 'markers', title: 'Finding intros and credits', started_at: now, done: 2, total: 5 },
      { id: 15, kind: 'previews', title: 'Making scrubbing previews', started_at: now, done: 3, total: 25, current: [
        { id: 9, stage: 'Making previews', subject: "Grey's Anatomy · S4 · E12", detail: 'greys.s04e12.mkv' },
      ] },
    ],
    recent: [
      { id: 9, kind: 'scan', title: 'Scanning TV Shows', started_at: now, finished_at: now, detail: '3 files checked · 1 identified' },
      { id: 8, kind: 'backup', title: 'Backing up the database', started_at: now, finished_at: now, error: 'Disk full' },
    ],
    scheduled: [{ kind: 'analysis', title: 'Update media analysis', state: 'paused', detail: 'Waiting for playback or scanning', pending: 70, completed: 5, failed: 0, enabled: true }],
  } }));
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto('/');
    await page.getByRole('button', { name: /^Activity/ }).click();
    const panel = page.locator('[data-radix-popper-content-wrapper]');
    await expect(panel.getByText('Scanning Movies', { exact: true })).toBeVisible();
    // One line per step under way, each with a single item.
    const scan = panel.getByRole('listitem').filter({ hasText: 'Scanning Movies' });
    await expect(scan.getByRole('term')).toHaveText(['Scanning', 'Matching', 'Analysing']);
    await expect(scan.getByRole('definition')).toHaveText(['Blade Runner (1982)', 'Dune (2021)', 'Arrival (2016)']);
    await expect(scan.getByText('Heat (1995)')).toHaveCount(0);
    await expect(panel.getByText('Finding intros and credits')).toHaveCount(0);
    await expect(panel.getByRole('listitem').filter({ hasText: 'Making scrubbing previews' })).toContainText("Grey's Anatomy · S4 · E12");
    const reading = panel.getByRole('listitem').filter({ hasText: 'Scanning TV Shows' });
    await expect(reading.getByRole('term')).toHaveText(['Analysing']);
    await expect(reading.getByRole('definition')).toHaveText(['3 of 40']);
    await expect(panel.getByText('3 of 12', { exact: true })).toBeVisible();
    await expect(panel.getByText('25%', { exact: true })).toBeVisible();
    // Internal steps, file names, scan statistics and scheduled backlogs stay on the Dashboard.
    for (const hidden of [/Matching movie/, /Inspecting file/, /Arrival\.2016\.mkv/, /files checked/, /Scheduled work/, /Update media analysis/]) {
      await expect(panel.getByText(hidden)).toHaveCount(0);
    }
    await expect(panel.getByText('Scanned TV Shows', { exact: true })).toBeVisible();
    await expect(panel.getByText('Backed up the database', { exact: true })).toBeVisible();
    await expect(panel.getByText('Disk full', { exact: true })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: info.outputPath(`activity-menu-${width}.png`), animations: 'disabled' });
    await page.keyboard.press('Escape');
  }
  await page.getByRole('button', { name: /^Activity/ }).click();
  await page.getByRole('button', { name: 'Stop Scanning Movies', exact: true }).click();
  await expect.poll(() => stopped).toBe(1);
  await page.getByRole('link', { name: 'View Dashboard', exact: true }).click();
  await expect(page).toHaveURL(/\/dashboard$/);
});

test('Activity distinguishes paused background work from running foreground work', async ({ page }) => {
  await seed(page);
  await page.route('**/api/activity', r => r.fulfill({ json: { running: [], recent: [], scheduled: [
    { kind: 'analysis', title: 'Update media analysis', state: 'paused', detail: 'Waiting for playback or scanning', pending: 70, completed: 5, failed: 0, enabled: true },
  ] } }));
  await page.goto('/dashboard');
  await expect(page.getByRole('heading', { name: 'Scheduled work', exact: true })).toBeVisible();
  await expect(page.getByText('Update media analysis', { exact: true })).toBeVisible();
  await expect(page.getByText('paused · Waiting for playback or scanning', { exact: true })).toBeVisible();
  await expect(page.getByText(/Nothing right now/)).toBeVisible();
});

test('poster menu: Watchlist changes show straight away and a slow action still reports back', async ({ page }) => {
  const { lib, item } = await seed(page);
  await page.goto('/library/' + lib.id);
  const card = page.locator(`main [data-item="${item.id}"]`);
  const more = card.getByRole('button', { name: `More for ${item.title}`, exact: true });
  await card.hover({ position: { x: 12, y: 60 } });
  await more.click();
  await page.getByRole('menuitem', { name: 'Add to Watchlist', exact: true }).click();
  await expect.poll(async () => (await (await page.request.get('/api/watchlist')).json()).some((i: Item) => i.id === item.id)).toBe(true);
  await card.hover({ position: { x: 12, y: 60 } });
  await more.click();
  await expect(page.getByRole('menuitem', { name: 'Remove from Watchlist', exact: true })).toBeVisible();
  await page.keyboard.press('Escape');
  // A show's Play next looks its episodes up first: its note arrives after the menu has closed.
  const shows = await seed(page, true);
  await page.goto('/library/' + shows.lib.id);
  const show = page.locator(`main [data-item="${shows.item.id}"]`);
  await page.route('**/api/series/*/seasons*', async (route) => { await new Promise((r) => setTimeout(r, 1500)); await route.continue(); });
  await show.hover({ position: { x: 12, y: 60 } });
  await show.getByRole('button', { name: `More for ${shows.item.title}`, exact: true }).click();
  await page.getByRole('menuitem', { name: 'Play next', exact: true }).click();
  await expect(page.getByRole('menu')).toHaveCount(0);
  await expect(page.getByRole('status').filter({ hasText: /Playback started|Added/ })).toBeVisible();
  await page.unroute('**/api/series/*/seasons*');
});


test('Dashboard distinguishes direct play, direct stream and transcoding', async ({ page }) => {
  const { item } = await seed(page);
  await page.route('**/api/playing', r => r.fulfill({ json: ['direct', 'remux', 'transcode'].map((method, i) => ({
    id: 'playing-' + i, item, profile: { id: 'admin', name: 'Admin', color: '#888' }, method, state: 'playing', device: 'Fixture', address: '127.0.0.1', started_at: 1,
    position_ms: 60_000, duration_ms: 120000, throttled: method === 'transcode',
    converted_ms: method === 'direct' ? undefined : 90_000,
    stream: method === 'direct'
      ? { video: 'HEVC 4K Dolby Vision', audio: 'Dolby Digital Plus 5.1' }
      : method === 'remux'
        ? { reasons: ['Dolby TrueHD audio'], video: 'HEVC 4K Dolby Vision', audio: 'Dolby TrueHD 7.1', audio_output: 'Dolby Digital Plus 5.1' }
        : { reasons: ['HEVC video'], video: 'HEVC 4K HDR10', video_output: 'H.264 1080p', audio: 'Dolby TrueHD 7.1', audio_output: 'AAC Stereo', encoder: 'Intel Quick Sync (VA-API)', hardware: true, tone_mapped: true },
  })) }));
  await page.goto('/dashboard');
  await expect(page.getByText('Direct Play', { exact: true })).toBeVisible();
  await expect(page.getByText('Direct Stream', { exact: true })).toBeVisible();
  await expect(page.getByText('Transcoding (throttled)', { exact: true })).toBeVisible();
  // Stream details: what's sent as it is, what's converted to what, why, by what, how far ahead.
  await expect(page.getByText('HEVC 4K Dolby Vision (as it is)', { exact: true }).first()).toBeVisible();
  await expect(page.getByText('Dolby TrueHD 7.1 → Dolby Digital Plus 5.1', { exact: true })).toBeVisible();
  await expect(page.getByText('HEVC 4K HDR10 → H.264 1080p', { exact: true })).toBeVisible();
  await expect(page.getByText('Hardware: Intel Quick Sync (VA-API)', { exact: true })).toBeVisible();
  await expect(page.getByText('HEVC video · HDR mapped to SDR', { exact: true })).toBeVisible();
  await expect(page.getByText('Converted to 1:30 (0:30 ahead)', { exact: true }).first()).toBeVisible();
  await page.screenshot({ path: test.info().outputPath('dashboard-stream-details.png'), fullPage: true });
});
