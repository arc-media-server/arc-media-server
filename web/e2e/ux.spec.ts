import { test, expect, type Page } from '@playwright/test';
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

test('settings groups expose labelled controls and persistent desktop and mobile navigation', async ({ page }, info) => {
  await seed(page);
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('/settings?section=library');
  const nav = page.getByRole('navigation', { name: 'Settings sections' });
  await expect(page.getByRole('heading', { name: 'Moving in', exact: true })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Notifications', exact: true })).toBeVisible();
  await expect(page.getByRole('combobox', { name: 'Region', exact: true })).toBeVisible();
  // Native links work with the keyboard and retain ordinary browser navigation semantics.
  const playback = nav.getByRole('link', { name: 'Playback & transcoding', exact: true });
  await playback.focus(); await page.keyboard.press('Enter');
  await expect(playback).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('heading', { name: 'Subtitle downloads', exact: true })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Moving in', exact: true })).toHaveCount(0);
  await nav.getByRole('link', { name: 'Converting', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Converting', exact: true })).toBeInViewport();
  await expect(playback).toBeInViewport();
  await expect(page.getByRole('combobox', { name: 'Videos converted at once', exact: true })).toBeVisible();
  await page.screenshot({ path: info.outputPath('settings-desktop.png') });
  await page.goBack(); // Subsection anchor, then group selection both participate in history.
  await page.goBack();
  await expect(page.getByRole('heading', { name: 'Moving in', exact: true })).toBeVisible();
  for (const group of ['profile', 'library', 'playback', 'access', 'maintenance']) {
    await page.goto('/settings?section=' + group);
    const selects = page.locator('main select:visible');
    for (const select of await selects.all()) await expect(select).toHaveAccessibleName(/.+/);
  }
  await expect(page.getByRole('link', { name: 'Backups & restore', exact: true })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Library Health', exact: true }).last()).toBeVisible();
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
  await expect(page.getByRole('heading', { name: 'Moving in', exact: true })).toBeVisible();
  await page.reload(); await expect(selector).toHaveValue('library');
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: info.outputPath('settings-mobile.png'), fullPage: true });
});

test('server settings keep saving, failure, retry and saved feedback beside the edited control', async ({ page }, info) => {
  await seed(page);
  await page.goto('/settings?section=playback');
  const control = page.getByRole('combobox', { name: 'Videos converted at once', exact: true });
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
  await expect(row.getByRole('status')).toHaveText('Saved');
  await expect(control).toHaveValue('2');
  await page.reload(); await expect(control).toHaveValue('2');
  // An async pre-roll save must also surface rejection rather than create an unhandled error.
  const errors: string[] = []; page.on('pageerror', error => errors.push(error.message));
  await page.route('**/api/settings', route => route.request().method() === 'PATCH'
    ? route.fulfill({ status: 503, json: { error: 'Pre-roll save failed' } }) : route.continue());
  const preroll = page.getByRole('combobox', { name: 'Pre-roll order', exact: true });
  await preroll.scrollIntoViewIfNeeded();
  await preroll.selectOption('all');
  await expect(page.getByRole('alert')).toContainText('Pre-roll save failed');
  await expect(page.getByRole('alert')).toBeInViewport();
  await page.unroute('**/api/settings');
  await page.getByRole('alert').getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByRole('combobox', { name: 'Pre-roll order', exact: true })).toHaveValue('all');
  await expect(page.getByRole('alert')).toHaveCount(0);
  await page.goto('/settings?section=access');
  const upload = page.getByRole('textbox', { name: 'Internet upload speed', exact: true });
  // Centre the control before saving; an auto-scrolled button at the bottom edge
  // leaves no room for the new inline feedback, independent of where it is rendered.
  await upload.evaluate(el => el.scrollIntoView({ block: 'center' }));
  await upload.fill('25');
  await page.route('**/api/settings', route => route.request().method() === 'PATCH'
    ? route.fulfill({ status: 503, json: { error: 'Upload save failed' } }) : route.continue());
  await upload.locator('..').getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('Upload save failed');
  await expect(page.getByRole('alert')).toBeInViewport();
  await page.unroute('**/api/settings');
  await page.getByRole('alert').getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByRole('alert')).toHaveCount(0);
  await page.reload(); await expect(upload).toHaveValue('25');
  expect(errors).toEqual([]);
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
  await page.getByRole('button', { name: /Aired Order/ }).click();
  await expect(page.getByText('Your choice also sets the server default')).toBeVisible();
  await page.getByRole('menuitemradio', { name: /Story Order/ }).click();
  await expect(page.getByText('Ordering save failed', { exact: true })).toBeVisible();
  await page.unroute('**/display-ordering');
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByRole('button', { name: /Story Order/ })).toBeVisible();
  await page.reload();
  await page.getByRole('button', { name: /Story Order/ }).click();
  await page.getByRole('menuitem', { name: 'Reset to source order', exact: true }).click();
  await expect(page.getByRole('button', { name: /Aired Order/ })).toBeVisible();
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
  for (const [tab, heading] of [['Libraries & metadata', 'Library scanning'], ['Playback & transcoding', 'Converting'], ['Access & devices', 'Devices'], ['Maintenance & diagnostics', 'Scheduled tasks']]) {
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

test('custom Home rows open complete filtered results and reorder by keyboard', async ({ page }) => {
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
  await expect(page.getByRole('combobox', { name: 'Length', exact: true })).toHaveValue('1');
  await page.goto('/?edit=1');
  const dialog = page.getByRole('dialog', { name: 'Edit Home', exact: true });
  const up = dialog.getByRole('button', { name: 'Move Watchlist up', exact: true });
  await up.focus(); await page.keyboard.press('Enter');
  await expect(dialog.getByRole('status')).toContainText('Watchlist, position 1');
  await dialog.getByRole('button', { name: 'Done', exact: true }).click();
  await expect(dialog).not.toBeVisible();
  await expect.poll(async () => (await (await page.request.get('/api/me/prefs')).json()).home[0].id).toBe('watchlist');
  await page.reload();
  await page.getByRole('button', { name: 'Edit Home', exact: true }).first().click();
  await expect(page.getByRole('button', { name: 'Move Watchlist up', exact: true })).toBeDisabled();
});

test('profile preference load failures offer recovery', async ({ page }) => {
  await page.route('**/api/me/prefs', r => r.fulfill({ status: 503, json: { error: 'Profile preferences unavailable' } }));
  await page.goto('/settings');
  await expect(page.getByText('Profile preferences unavailable', { exact: true })).toBeVisible();
  await page.unroute('**/api/me/prefs');
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByRole('switch', { name: 'Hide spoilers', exact: true })).toBeVisible();
});

test('queue editing preserves current playback, browsing and saved episode progress', async ({ page }, info) => {
  const shows = await seed(page, true); await seed(page);
  let starts = 0;
  page.on('request', r => { if (r.method() === 'POST' && r.url().endsWith('/playback')) starts++; });
  await page.goto('/series/' + shows.item.id);
  await page.getByRole('button', { name: /^Play S/ }).click();
  const video = page.locator('video');
  const cdp = await page.context().newCDPSession(page); await cdp.send('Emulation.setFocusEmulationEnabled', { enabled: true });
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(0);
  for (let i = 0; i < 7; i++) await page.keyboard.press('l');
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(65);
  await page.keyboard.press('Space');
  await page.getByRole('button', { name: 'Minimize player', exact: true }).click();
  await page.locator('aside').getByRole('link', { name: 'Audit Movies', exact: true }).click();
  await page.getByRole('link', { name: /Arc Movie/ }).first().click();
  await page.getByRole('button', { name: 'Queue Arc Movie', exact: true }).click();
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
  expect(starts).toBe(1);
  expect(await video.evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(65);
  await page.screenshot({ path: info.outputPath('editable-queue.png') });
  await queue.getByRole('button', { name: 'Clear upcoming', exact: true }).click();
  await expect(queue.locator('ol li')).toHaveCount(1);
  await queue.getByRole('button', { name: 'Close', exact: true }).click();
  await page.getByRole('button', { name: 'Close player', exact: true }).click();
  await expect.poll(async () => (await (await page.request.get('/api/home/continue')).json()).some((i: Item) => i.title === 'First Chapter' && (i.user_data?.position_ms || 0) > 65_000)).toBe(true);
});

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

test('comparison keeps selections on failure and profiles can return to inherited order', async ({ page }) => {
  const { item } = await seed(page, true);
  await page.goto('/series/' + item.id);
  await page.getByRole('button', { name: /Aired Order/ }).click();
  await page.getByRole('menuitem', { name: /Compare Orders/ }).click();
  const dialog = page.getByRole('dialog', { name: 'Compare Episode Orders', exact: true });
  await page.route('**/display-ordering', r => r.fulfill({ status: 503, json: { error: 'Comparison save unavailable' } }));
  await dialog.getByRole('button', { name: 'Use Story Order', exact: true }).click();
  await expect(dialog.getByText('Comparison save unavailable', { exact: true })).toBeVisible();
  await expect(dialog).toBeVisible();
  await page.unroute('**/display-ordering');
  await dialog.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(dialog).not.toBeVisible();
  await expect(page.getByRole('button', { name: /Story Order/ })).toBeVisible();
  const viewer = await (await page.request.post('/api/profiles', { data: { name: 'Order viewer', color: 'blue' } })).json();
  expect((await page.request.post('/api/auth/profile', { data: { id: viewer.id } })).ok()).toBe(true);
  await page.reload();
  await page.getByRole('button', { name: /Story Order/ }).click();
  await expect(page.getByText('Your profile only', { exact: true })).toBeVisible();
  await page.getByRole('menuitemradio', { name: /Aired Order/ }).click();
  await expect(page.getByRole('button', { name: /Aired Order/ })).toBeVisible();
  await page.getByRole('button', { name: /Aired Order/ }).click();
  await page.getByRole('menuitem', { name: 'Use server default', exact: true }).click();
  await expect(page.getByRole('button', { name: /Story Order/ })).toBeVisible();
});

test('ratings, watchlist and metadata refresh failures explain and recover', async ({ page }) => {
  const { item } = await seed(page);
  await page.goto('/movie/' + item.id);
  for (const [suffix, action] of [['rating', '5 stars'], ['watchlist', 'Add to Watchlist'], ['refresh', 'Refresh']]) {
    await page.route('**/' + suffix, r => r.fulfill({ status: 503, json: { error: suffix + ' unavailable' } }));
    await page.getByRole(suffix === 'rating' ? 'radio' : 'button', { name: action, exact: true }).click();
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

test('browse sort direction is remembered per profile on this device', async ({ page }) => {
  const { lib } = await seed(page);
  await page.goto('/library/' + lib.id);
  const original = (await (await page.request.get('/api/items?library_id=' + lib.id + '&sort=title')).json()).items.map((i: Item) => i.id);
  const reversed = (await (await page.request.get('/api/items?library_id=' + lib.id + '&sort=title&reverse=1')).json()).items.map((i: Item) => i.id);
  expect(reversed).toEqual([...original].reverse());
  await page.getByRole('button', { name: 'Reverse sort direction', exact: true }).click();
  await expect(page).toHaveURL(/reverse=1/);
  await page.goto('/library/' + lib.id);
  await expect(page.getByRole('button', { name: 'Reverse sort direction', exact: true })).toHaveAttribute('aria-pressed', 'true');
  const viewer = await (await page.request.post('/api/profiles', { data: { name: 'Browse viewer', color: 'blue' } })).json();
  await page.request.post('/api/auth/profile', { data: { id: viewer.id } });
  await page.goto('/library/' + lib.id);
  await expect(page.getByRole('button', { name: 'Reverse sort direction', exact: true })).toHaveAttribute('aria-pressed', 'false');
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
  await page.getByRole('button', { name: /^Play S/ }).click();
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

test('diagnostics can be enabled, downloaded and retained across restart with admin protection', async ({ page }, info) => {
  await page.goto('/settings?section=maintenance');
  const toggle = page.getByRole('switch', { name: 'Detailed logging', exact: true });
  await expect(toggle).not.toBeChecked();
  await page.route('**/api/diagnostics', r => r.request().method() === 'PATCH' ? r.fulfill({ status: 503, json: { error: 'Diagnostics unavailable' } }) : r.continue());
  await toggle.click();
  await expect(page.getByText('Diagnostics unavailable', { exact: true })).toBeVisible();
  await page.unroute('**/api/diagnostics');
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
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

test('source labels keep HDR and profile-based audio with their own movie version', async ({ page }, info) => {
  const { item } = await seed(page);
  const original = await (await page.request.get('/api/items/' + item.id)).json();
  const base = original.files[0];
  const file = (id: number, edition: string, plus: boolean, dv: boolean, audioProfile: string) => ({ ...base, id, edition, streams: [
    { index: 0, kind: 'video', codec: 'hevc', width: 3840, height: 2160, is_default: true, is_forced: false, color_transfer: 'smpte2084', dolby_vision: dv, hdr10_plus: plus, hdr_probe_status: 'sampled' },
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
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: info.outputPath(`source-formats-${width}.png`), fullPage: true });
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
  const button = page.getByRole('button', { name: /^File details for / }).first();
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
  const dialog = page.getByRole('dialog', { name: /^File details:/ });
  await expect(dialog.getByText(/1080p · HEVC · HLG/)).toBeVisible();
  await expect(dialog.getByText('Dolby Digital+ Atmos · 5.1', { exact: true })).toBeVisible();
  await expect(dialog.getByText('DTS-HD MA · DTS:X · 7.1', { exact: true })).toBeVisible();
  await expect(dialog.getByText('Dolby TrueHD · 7.1', { exact: true })).toBeVisible();
  await expect(dialog.getByText('Dolby Vision', { exact: true })).toHaveCount(0);
  await expect(dialog.getByText(/Source formats; playback output depends/)).toBeVisible();
  await page.keyboard.press('Escape'); await expect(dialog).toHaveCount(0);
});

test('background scheduling exposes modes, task controls and recoverable Run now', async ({ page }, info) => {
  await seed(page);
  await page.goto('/settings?section=library');
  const interval = page.getByRole('combobox', { name: 'Full library scan', exact: true });
  await expect(interval.getByRole('option', { name: 'Every 15 minutes', exact: true })).toHaveCount(1);
  await expect(interval.getByRole('option', { name: 'Every 30 minutes', exact: true })).toHaveCount(1);
  await expect(interval.getByRole('option', { name: 'Every 2 hours', exact: true })).toHaveCount(1);
  await interval.selectOption('15'); await expect(interval).toBeEnabled();
  await page.reload(); await expect(interval).toHaveValue('15');
  expect((await (await page.request.get('/api/settings')).json()).scan_interval_minutes).toBe(15);
  await page.goto('/settings?section=playback');
  const detection = page.getByRole('combobox', { name: 'Local intro and credits detection', exact: true });
  await detection.selectOption('off'); await expect(detection).toBeEnabled();
  await page.reload(); await expect(detection).toHaveValue('off');
  await page.goto('/settings?section=maintenance');
  const preview = page.getByRole('combobox', { name: 'Scrubbing preview processing', exact: true });
  await preview.selectOption('scheduled'); await expect(preview).toBeEnabled();
  const analysis = page.getByRole('switch', { name: 'Update existing media analysis', exact: true });
  await analysis.click(); await expect(analysis).not.toBeChecked();
  const run = page.getByRole('button', { name: 'Run update media analysis now', exact: true });
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

test('Activity distinguishes paused background work from running foreground work', async ({ page }) => {
  await seed(page);
  await page.route('**/api/activity', r => r.fulfill({ json: { running: [], recent: [], scheduled: [
    { kind: 'analysis', title: 'Update media analysis', state: 'paused', detail: 'Waiting for playback or scanning', pending: 70, completed: 5, failed: 0, enabled: true },
  ] } }));
  await page.goto('/');
  await page.getByRole('button', { name: 'Activity', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Scheduled work', exact: true })).toBeVisible();
  await expect(page.getByText('Update media analysis', { exact: true })).toBeVisible();
  await expect(page.getByText('paused · Waiting for playback or scanning', { exact: true })).toBeVisible();
  await expect(page.getByText(/Nothing right now/)).toBeVisible();
});
