import { test as base, expect, type Page } from '@playwright/test';
import type { Item, Library } from '../src/lib/api';

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
  const lib = await response.json();
  await expect.poll(async () => (await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json()).running).toBe(false);
  return (await (await page.request.get(`/api/items?library_id=${lib.id}`)).json()).items;
}


async function closePlayer(page: Page) {
  await page.getByRole('button', { name: 'Close player', exact: true }).click();
  await expect(page.locator('video')).toHaveCount(0);
  await expect(page.getByRole('region', { name: 'Media player', exact: true })).toHaveCount(0);
}

async function focusedPlayback(page: Page) {
  const cdp = await page.context().newCDPSession(page);
  await cdp.send('Emulation.setFocusEmulationEnabled', { enabled: true });
}

test('web server: library order saves per profile, retries and never grants library access', async ({ page, media }, info) => {
  const libs: Library[] = [];
  for (const [name, kind, path] of [['Films', 'movies', media.movies], ['Series', 'shows', media.shows]]) {
    const response = await page.request.post('/api/libraries', { data: { name, kind, paths: [path] } });
    expect(response.ok()).toBeTruthy();
    const lib = await response.json(); libs.push(lib);
    await expect.poll(async () => (await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json()).running).toBe(false);
  }
  await page.goto('/settings');
  const region = page.getByRole('region', { name: 'Library navigation order' });
  const original = await page.locator('aside nav a[href^="/library/"]').allTextContents();
  expect(original).toEqual(['Films', 'Series']);
  let fail = true;
  await page.route('**/api/me/prefs', route => route.request().method() === 'PATCH' && fail && route.request().postDataJSON().library_order
    ? route.fulfill({ status: 503, json: { error: 'Order save unavailable' } }) : route.continue());
  await region.getByRole('button', { name: 'Move Series up', exact: true }).click();
  await expect(region.getByRole('alert')).toContainText('Order save unavailable');
  await expect.poll(() => page.locator('aside nav a[href^="/library/"]').allTextContents()).toEqual(original);
  expect((await (await page.request.get('/api/me/prefs')).json()).library_order).toBeUndefined();
  fail = false;
  await region.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect.poll(() => page.locator('aside nav a[href^="/library/"]').allTextContents()).toEqual(['Series', 'Films']);
  await page.reload();
  await expect.poll(async () => (await (await page.request.get('/api/me/prefs')).json()).library_order).toEqual([libs[1].id, libs[0].id]);
  await page.screenshot({ path: info.outputPath('library-order-desktop.png') });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect.poll(() => page.locator('nav:visible a[href^="/library/"]').allTextContents()).toEqual(['Series', 'Films']);
  await region.scrollIntoViewIfNeeded();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: info.outputPath('library-order-phone.png'), fullPage: true });
  const response = await page.request.post('/api/profiles', { data: { name: 'Limited', library_ids: [libs[0].id] } });
  expect(response.ok()).toBeTruthy();
  const viewer = await response.json();
  expect((await page.request.post('/api/auth/profile', { data: { id: viewer.id } })).ok()).toBeTruthy();
  // An inaccessible ID in the preference must not add that library to navigation.
  expect((await page.request.patch('/api/me/prefs', { data: { library_order: [libs[1].id, libs[0].id] } })).ok()).toBeTruthy();
  await page.reload();
  await expect.poll(() => page.locator('nav:visible a[href^="/library/"]').allTextContents()).toEqual(['Films']);
  expect((await (await page.request.get(`/api/items?library_id=${libs[1].id}`)).json()).items).toEqual([]);
  await region.getByRole('button', { name: 'Reset library order', exact: true }).click();
  await expect.poll(async () => (await (await page.request.get('/api/me/prefs')).json()).library_order ?? []).toEqual([]);
});

test('web server: watched threshold preserves resume until the selected boundary and keeps old history', async ({ page, media }, info) => {
  const items: Item[] = await addLibrary(page, media.movies);
  const item = items[0];
  await page.goto('/settings');
  const threshold = page.getByRole('combobox', { name: 'Mark as watched at', exact: true });
  await expect(threshold).toHaveValue('90');
  await threshold.selectOption('95');
  await expect.poll(async () => (await (await page.request.get('/api/me/prefs')).json()).watched_percent).toBe(95);
  await page.reload();
  await expect(threshold).toHaveValue('95');
  const report = async (position: number) => {
    const r = await page.request.put(`/api/items/${item.id}/progress`, { data: { position_ms: position, duration_ms: 120000, state: 'paused' } });
    expect(r.ok()).toBeTruthy(); return r.json();
  };
  expect(await report(108000)).toMatchObject({ played: false, position_ms: 108000 });
  expect(await report(114000)).toMatchObject({ played: true, play_count: 1 });
  expect(await report(114000)).toMatchObject({ played: true, play_count: 1 });
  await threshold.selectOption('100');
  await expect.poll(async () => (await (await page.request.get('/api/me/prefs')).json()).watched_percent).toBe(100);
  expect((await (await page.request.get(`/api/items/${item.id}`)).json()).user_data.played).toBe(true);
  expect((await page.request.put(`/api/items/${item.id}/played`, { data: { played: false } })).ok()).toBeTruthy();
  expect(await report(119999)).toMatchObject({ played: false, position_ms: 119999 });
  // Marking unwatched retains earlier plays; a new completed viewing adds one.
  expect(await report(120000)).toMatchObject({ played: true, play_count: 2 });
  await page.screenshot({ path: info.outputPath('watched-threshold.png') });
});

test('web server: text subtitle timing shifts actual cues, survives seeking and resets by track/session', async ({ page, media }, info) => {
  const items: Item[] = await addLibrary(page, media.movies);
  const item = items.find(i => i.title === 'Arc Movie')!;
  await page.request.patch('/api/me/prefs', { data: { subtitle_mode: 'manual' } });
  await page.goto(`/movie/${item.id}`);
  await focusedPlayback(page);
  await page.getByRole('button', { name: 'Play', exact: true }).click();
  const player = page.getByRole('region', { name: 'Media player', exact: true });
  await expect(page.locator('video')).toBeVisible();
  const menu = async () => { await player.hover(); await player.getByRole('button', { name: 'Subtitles (C)', exact: true }).click(); };
  await menu();
  await page.getByRole('menuitem').filter({ hasText: 'English' }).click();
  await expect(page.locator('[data-subtitles]')).toHaveText('English fixture subtitle');
  await page.locator('video').evaluate((v: HTMLVideoElement) => { v.pause(); v.currentTime = 0.25; });
  await menu();
  await page.getByRole('menuitem', { name: 'Show 0.5s later', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Show 0.5s later', exact: true }).click();
  await expect(page.locator('[data-subtitles]')).toHaveCount(0);
  await expect(page.getByText('Timing: +1.0s (later)', { exact: true })).toBeVisible();
  await page.locator('video').evaluate((v: HTMLVideoElement) => { v.currentTime = 2; });
  await expect(page.locator('[data-subtitles]')).toHaveText('English fixture subtitle');
  await page.locator('video').evaluate((v: HTMLVideoElement) => { v.currentTime = 0.25; });
  await expect(page.locator('[data-subtitles]')).toHaveCount(0);
  // Exercise real native TextTrack/VTTCue wiring; no physical floating window is claimed.
  await page.locator('video').dispatchEvent('enterpictureinpicture');
  await expect.poll(() => page.locator('video').evaluate((v: HTMLVideoElement) => Array.from(v.textTracks).find(t => t.mode === 'showing')?.cues?.[0]?.startTime)).toBe(1);
  await page.getByRole('menuitem', { name: 'Reset subtitle timing', exact: true }).click();
  await expect.poll(() => page.locator('video').evaluate((v: HTMLVideoElement) => Array.from(v.textTracks).find(t => t.mode === 'showing')?.cues?.[0]?.startTime)).toBe(0);
  await page.locator('video').dispatchEvent('leavepictureinpicture');
  await expect(page.locator('[data-subtitles]')).toHaveText('English fixture subtitle');
  await page.getByRole('menuitem', { name: 'Show 0.5s later', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Show 0.5s later', exact: true }).click();
  await page.getByRole('menuitem').filter({ hasText: 'French' }).click();
  await expect(page.locator('[data-subtitles]')).toHaveText('French fixture subtitle');
  await menu();
  await page.getByRole('menuitem').filter({ hasText: 'English' }).click();
  await expect(page.locator('[data-subtitles]')).toHaveCount(0);
  await menu();
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({ path: info.outputPath('subtitle-timing-phone.png') });
  await page.keyboard.press('Escape');
  await closePlayer(page);
  await page.getByRole('button', { name: 'Play', exact: true }).click();
  await menu();
  await page.getByRole('menuitem').filter({ hasText: 'English' }).click();
  await expect(page.locator('[data-subtitles]')).toHaveText('English fixture subtitle');
  await menu();
  await expect(page.getByRole('menuitem', { name: 'Reset subtitle timing', exact: true })).toBeDisabled();
});

async function receiverFixture(page: Page) {
  // SDK/receiver stand-in only: no external scripts, real device or output claim.
  await page.addInitScript(() => {
    const w = window as any;
    w.__receiverLoads = [];
    w.__receiverChoices = 0;
    w.__airplayChoices = 0;
    w.WebKitPlaybackTargetAvailabilityEvent = Event;
    Object.defineProperty(HTMLVideoElement.prototype, 'webkitShowPlaybackTargetPicker', { configurable: true, value() { w.__airplayChoices++; } });
    class RemotePlayer { isConnected = true; isMediaLoaded = true; isPaused = false; currentTime = 0; }
    class Controller {
      handlers: Record<string, () => void> = {};
      constructor(public player: RemotePlayer) { w.__receiverController = this; }
      addEventListener(type: string, fn: () => void) { this.handlers[type] = fn; }
      removeEventListener(type: string) { delete this.handlers[type]; }
      seek() {} playOrPause() { this.player.isPaused = !this.player.isPaused; }
    }
    const session = { getCastDevice: () => ({ friendlyName: 'Local receiver fixture' }), loadMedia: async (req: any) => { w.__receiverLoads.push({ url: req.media.contentId, time: req.currentTime }); } };
    const ctx = { setOptions() {}, getCastState: () => 'CONNECTED', addEventListener() {}, removeEventListener() {}, requestSession: async () => { w.__receiverChoices++; }, getCurrentSession: () => session, endCurrentSession() {} };
    w.cast = { framework: { CastContext: { getInstance: () => ctx }, CastContextEventType: { CAST_STATE_CHANGED: 'state' }, AutoJoinPolicy: { ORIGIN_SCOPED: 'origin' }, RemotePlayer, RemotePlayerController: Controller, RemotePlayerEventType: { CURRENT_TIME_CHANGED: 'time', IS_PAUSED_CHANGED: 'paused', IS_CONNECTED_CHANGED: 'connected' } } };
    w.chrome ??= {};
    w.chrome.cast = { AutoJoinPolicy: { ORIGIN_SCOPED: 'origin' }, media: {
      DEFAULT_MEDIA_RECEIVER_APP_ID: 'fixture',
      MediaInfo: class { constructor(public contentId: string, public contentType: string) {} },
      GenericMediaMetadata: class {}, LoadRequest: class { constructor(public media: unknown) {} },
      StreamType: { BUFFERED: 'buffered' }, HlsSegmentFormat: { FMP4: 'fmp4' }, HlsVideoSegmentFormat: { FMP4: 'fmp4' },
    } };
  });
  await page.route('https://www.gstatic.com/cv/js/sender/**', route => route.fulfill({ contentType: 'application/javascript', body: 'window.__onGCastApiAvailable(true);' }));
}

test('web server: cast retry and converted seek retain selected file/audio and release failed conversions', async ({ page, media }, info) => {
  await receiverFixture(page);
  const [item] = await addLibrary(page, media.preplay);
  const detail = await (await page.request.get(`/api/items/${item.id}`)).json();
  const director = detail.files.find((f: { path: string }) => f.path.includes("Director's Cut"));
  let collect = false, localOnly = true;
  const plans: { request: Record<string, number>; response: { session_id?: string; offset_ms: number; file_id: number; audio_stream: number } }[] = [];
  const deleted: string[] = [];
  const released: string[] = [];
  page.on('request', r => { if (r.method() === 'DELETE' && r.url().includes('/api/transcode/')) deleted.push(r.url().split('/').pop()!); });
  page.on('response', r => { if (r.request().method() === 'DELETE' && r.url().includes('/api/transcode/') && r.status() === 204) released.push(r.url().split('/').pop()!); });
  await page.route('**/api/items/*/playback', async route => {
    const response = await route.fetch(); const data = await response.json();
    if (collect) plans.push({ request: route.request().postDataJSON(), response: data });
    return route.fulfill({ response, json: data });
  });
  await page.route('**/api/cast', route => route.fulfill({ json: { url: 'http://127.0.0.1:18199/receiver-fixture', local_only: localOnly } }));
  await page.goto(`/movie/${item.id}`);
  await focusedPlayback(page);
  await page.getByRole('button', { name: 'Playback options', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'Playback options', exact: true });
  await dialog.getByRole('combobox', { name: 'Version', exact: true }).selectOption(String(director.id));
  const audio = dialog.getByRole('combobox', { name: 'Audio', exact: true });
  const french = Number(await audio.getByRole('option', { name: /^French/ }).getAttribute('value'));
  await audio.selectOption(String(french));
  await dialog.getByRole('combobox', { name: 'Subtitles', exact: true }).selectOption('off');
  await dialog.getByRole('button', { name: 'Play', exact: true }).click();
  await expect.poll(() => page.locator('video').evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(1);
  const player = page.getByRole('region', { name: 'Media player', exact: true });
  await player.hover();
  collect = true;
  await player.getByRole('button', { name: 'Cast to a Chromecast', exact: true }).click();
  await expect(player.getByRole('alert')).toContainText('Only this computer can reach Arc');
  expect(plans).toHaveLength(1);
  expect(plans[0].response.session_id).toBeTruthy();
  expect(deleted).toContain(plans[0].response.session_id);
  localOnly = false;
  await player.getByRole('button', { name: 'Retry casting', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).__receiverLoads.length)).toBe(1);
  await expect(player.getByText('Playing on Local receiver fixture', { exact: true })).toBeVisible();
  await player.getByRole('button', { name: 'Forward 10 seconds', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).__receiverLoads.length)).toBe(2);
  expect(plans).toHaveLength(3);
  for (const plan of plans) {
    expect(plan.request).toMatchObject({ file_id: director.id, audio_stream: french });
    expect(plan.response).toMatchObject({ file_id: director.id, audio_stream: french });
  }
  expect(plans[2].request.start_ms).toBeGreaterThan(plans[1].request.start_ms);
  expect(deleted).toContain(plans[1].response.session_id);
  const latest = plans[2];
  const loads = await page.evaluate(() => (window as any).__receiverLoads);
  expect(loads[1].time).toBeCloseTo(Math.max(0, (latest.request.start_ms - latest.response.offset_ms) / 1000), 2);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({ path: info.outputPath('cast-source-phone.png') });
  collect = false;
  const resumed = page.waitForResponse(r => r.url().endsWith('/playback') && r.ok());
  await player.getByRole('button', { name: 'Stop Casting', exact: true }).click();
  const handback = await resumed;
  expect(handback.request().postDataJSON()).toMatchObject({ file_id: director.id, audio_stream: french });
  expect(released).toContain(latest.response.session_id);
  await expect(page.locator('video')).toBeVisible();
  await expect.poll(() => page.locator('video').evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(1);
  await expect.poll(() => deleted.includes(latest.response.session_id!)).toBe(true);
  await player.hover();
  collect = true;
  await player.getByRole('button', { name: 'Cast to a Chromecast', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).__receiverLoads.length)).toBe(3);
  const disconnectedSession = plans[3].response.session_id;
  collect = false;
  const afterDisconnect = page.waitForResponse(r => r.url().endsWith('/playback') && r.ok());
  await page.evaluate(() => { const c = (window as any).__receiverController; c.player.isConnected = false; c.handlers.connected(); });
  expect((await afterDisconnect).request().postDataJSON()).toMatchObject({ file_id: director.id, audio_stream: french });
  expect(released).toContain(disconnectedSession);
  await expect.poll(() => page.locator('video').evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(1);
  await closePlayer(page);
});

test('web server: casting text subtitles explains the limit without silently dropping them', async ({ page, media }) => {
  await receiverFixture(page);
  const items: Item[] = await addLibrary(page, media.movies);
  const item = items.find(i => i.title === 'Arc Movie')!;
  let links = 0;
  await page.route('**/api/cast', route => { links++; return route.fulfill({ json: { url: 'http://127.0.0.1:18199/receiver-fixture', local_only: false } }); });
  await page.goto(`/movie/${item.id}`);
  await page.getByRole('button', { name: 'Play', exact: true }).click();
  const player = page.getByRole('region', { name: 'Media player', exact: true });
  await player.hover();
  await player.getByRole('button', { name: 'Subtitles (C)', exact: true }).click();
  await page.getByRole('menuitem').filter({ hasText: 'English' }).click();
  await expect(page.locator('[data-subtitles]')).toHaveText('English fixture subtitle');
  await page.locator('video').evaluate(v => v.dispatchEvent(Object.assign(new Event('webkitplaybacktargetavailabilitychanged'), { availability: 'available' })));
  await player.hover();
  await player.getByRole('button', { name: 'Cast to a Chromecast', exact: true }).click();
  await expect(player.getByRole('status')).toContainText('Chromecast cannot receive browser-rendered text subtitles yet');
  expect(links).toBe(0);
  expect(await page.evaluate(() => (window as any).__receiverChoices)).toBe(0);
  await expect(page.locator('video')).toBeVisible();
  await expect(page.locator('[data-subtitles]')).toHaveText('English fixture subtitle');
  await player.hover();
  await player.getByRole('button', { name: 'AirPlay', exact: true }).click();
  await expect(player.getByRole('status')).toContainText('AirPlay cannot receive browser-rendered text subtitles yet');
  expect(await page.evaluate(() => (window as any).__airplayChoices)).toBe(0);
  await player.hover();
  await player.getByRole('button', { name: 'Subtitles (C)', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Off', exact: true }).click();
  await player.hover();
  await player.getByRole('button', { name: 'Cast to a Chromecast', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).__receiverLoads.length)).toBe(1);
  await page.evaluate(() => { const c = (window as any).__receiverController; c.player.currentTime = 73; c.handlers.time(); });
  await closePlayer(page);
  await expect.poll(async () => (await (await page.request.get(`/api/items/${item.id}`)).json()).user_data.position_ms).toBe(73000);
});
