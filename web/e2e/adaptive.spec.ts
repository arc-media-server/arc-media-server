import { test as base, expect, type Page } from '@playwright/test';
import type { PlaybackPlan } from '../src/lib/api';

type Admission = { request: { file_id?: number; audio_stream?: number; start_ms: number; max_kbps?: number }; response: PlaybackPlan };
const test = base.extend<{ movie: string }>({
  movie: async ({ page }, use) => {
    expect((await fetch('http://127.0.0.1:18200/reset', { method: 'POST' })).ok).toBe(true);
    const errors: string[] = []; page.on('pageerror', e => errors.push(e.stack ?? e.message));
    await page.context().route('**/*', route => {
      const url = new URL(route.request().url());
      return url.hostname === '127.0.0.1' || url.protocol === 'data:' ? route.continue() : route.abort();
    });
    expect((await page.request.post('/api/auth/setup', { data: { name: 'Owner', password: 'Adaptive-fixture-123', local_bypass: false, tmdb_api_key: 'local-fixture-key', region: 'GB' } })).ok()).toBeTruthy();
    expect((await page.request.patch('/api/settings', { data: { scan_automatic: false, scan_on_start: false, scan_interval_minutes: 0, preview_mode: 'off', detection_mode: 'off', analysis_enabled: false, metadata_refresh_enabled: false, auto_backups_enabled: false } })).ok()).toBeTruthy();
    expect((await page.request.patch('/api/me/prefs', { data: { audio_language: 'fra', subtitle_language: 'fra', subtitle_mode: 'always' } })).ok()).toBeTruthy();
    const media = await (await page.request.post('http://127.0.0.1:18200/adaptive-media', { timeout: 120000 })).json();
    const lib = await (await page.request.post('/api/libraries', { data: { name: 'Adaptive fixture', kind: 'movies', paths: [media.path] } })).json();
    await expect.poll(async () => (await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json()).running).toBe(false);
    const items = await (await page.request.get(`/api/items?library_id=${lib.id}`)).json();
    expect(items.items).toHaveLength(1);
    await use(items.items[0].id);
    expect(errors, 'uncaught browser errors').toEqual([]);
  },
});

async function openPlayer(page: Page, movie: string) {
  await page.goto(`/movie/${movie}`);
  const cdp = await page.context().newCDPSession(page);
  await cdp.send('Emulation.setFocusEmulationEnabled', { enabled: true });
  await page.getByRole('button', { name: 'Play', exact: true }).click();
  const video = page.locator('video');
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime), { timeout: 30000 }).toBeGreaterThan(0.5);
  await expect(page.locator('[data-subtitles]')).toHaveText('Adaptive French subtitle');
  return { cdp, video, player: page.getByRole('region', { name: 'Media player', exact: true }) };
}

test('adaptive: constrained real delivery lowers quality, preserves source/audio/captions and manual Original takes over', async ({ page, movie }, info) => {
  test.setTimeout(120000);
  const plans: Admission[] = [];
  await page.route('**/api/items/*/playback', async route => {
    const response = await route.fetch(); const data = await response.json();
    plans.push({ request: route.request().postDataJSON(), response: data });
    await route.fulfill({ response, json: data });
  });
  const network = await page.context().newCDPSession(page);
  await network.send('Network.enable');
  await network.send('Network.emulateNetworkConditions', { offline: false, latency: 10, downloadThroughput: 1250000, uploadThroughput: -1 });
  const { video, player } = await openPlayer(page, movie);
  const original = plans[0].response;
  expect(original.source_kbps).toBeGreaterThan(3000);
  expect(original.audio_tracks.find(a => a.index === original.audio_stream)?.language).toMatch(/fra|fre/);
  await network.send('Network.emulateNetworkConditions', { offline: false, latency: 30, downloadThroughput: 250000, uploadThroughput: -1 });
  await expect.poll(() => plans.some(p => p.request.max_kbps && p.request.max_kbps <= 1500), { timeout: 80000 }).toBe(true);
  const adjusted = plans.filter(p => p.request.max_kbps);
  expect(adjusted.length).toBeLessThanOrEqual(4);
  for (const admission of adjusted) {
    expect(admission.request).toMatchObject({ file_id: original.file_id, audio_stream: original.audio_stream });
    expect(admission.request.start_ms).toBeGreaterThan(0);
    expect(admission.response.file_id).toBe(original.file_id);
  }
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => !v.paused && v.readyState >= 2), { timeout: 30000 }).toBe(true);
  await expect(page.locator('[data-subtitles]')).toHaveText('Adaptive French subtitle');
  await player.hover(); await player.getByRole('button', { name: 'Quality', exact: true }).click();
  await expect(page.getByRole('menuitemradio', { name: 'Automatic', exact: true })).toBeChecked();
  await page.screenshot({ path: info.outputPath('automatic-quality-limit.png') });
  await network.send('Network.emulateNetworkConditions', { offline: false, latency: 0, downloadThroughput: -1, uploadThroughput: -1 });
  const restored = page.waitForResponse(r => r.url().endsWith('/playback') && r.ok());
  await page.getByRole('menuitemradio', { name: 'Original', exact: true }).click();
  expect((await restored).request().postDataJSON().max_kbps).toBeUndefined();
  expect(await page.evaluate(() => localStorage.getItem('arc-quality'))).toBe('0');
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.readyState >= 2)).toBe(true);
  await player.getByRole('button', { name: 'Close player', exact: true }).click();
});

test('adaptive: pause during replan, exact tracks, seeking and readmission failure remain recoverable', async ({ page, movie }) => {
  test.setTimeout(90000);
  // Install before playback creates timers. Installing the clock mid-playback leaves
  // native HLS reload timers outside the emulated clearTimeout during teardown.
  await page.clock.install();
  let hold = false, fail = false; let release!: () => void;
  let held!: () => void; const entered = new Promise<void>(resolve => { held = resolve; });
  const gate = new Promise<void>(resolve => { release = resolve; });
  const plans: Admission[] = [];
  await page.route('**/api/items/*/playback', async route => {
    if (hold && route.request().postDataJSON().max_kbps) { held(); await gate; }
    if (fail) return route.fulfill({ status: 503, json: { error: 'Adaptive admission unavailable' } });
    const response = await route.fetch(); const data = await response.json();
    plans.push({ request: route.request().postDataJSON(), response: data });
    await route.fulfill({ response, json: data });
  });
  const { video, player } = await openPlayer(page, movie);
  const original = plans[0].response;
  await player.hover(); await player.getByRole('button', { name: 'Speed', exact: true }).click();
  await page.getByRole('menuitem', { name: '1.25×', exact: true }).click();
  await player.hover(); await player.getByRole('button', { name: 'Subtitles (C)', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Show 0.5s later', exact: true }).click();
  await page.keyboard.press('Escape');
  // Deterministic pressure fixture: actual planner/media, simulated buffer/readiness and elapsed policy time.
  const now = await page.evaluate(() => Date.now());
  await video.evaluate((v: HTMLVideoElement) => {
    Object.defineProperty(v, 'buffered', { configurable: true, get: () => ({ length: 0 }) });
    Object.defineProperty(v, 'readyState', { configurable: true, get: () => 2 });
  });
  hold = true;
  await page.clock.setFixedTime(new Date(now + 13000));
  await page.waitForTimeout(1500); // one real polling interval with sustained pressure
  await page.clock.setFixedTime(new Date(now + 21000));
  await entered;
  await player.hover(); await player.getByRole('button', { name: 'Pause (Space)', exact: true }).click();
  await video.evaluate(v => { delete (v as unknown as { buffered?: unknown }).buffered; delete (v as unknown as { readyState?: unknown }).readyState; });
  release(); hold = false;
  await expect.poll(() => plans.length).toBe(2);
  expect(plans[1].request).toMatchObject({ file_id: original.file_id, audio_stream: original.audio_stream });
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.paused)).toBe(true);
  expect(await video.evaluate((v: HTMLVideoElement) => v.playbackRate)).toBe(1.25);
  await player.getByRole('button', { name: 'Play (Space)', exact: true }).click();
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => !v.paused && v.readyState >= 2)).toBe(true);
  await expect(page.locator('[data-subtitles]')).toHaveText('Adaptive French subtitle');
  await player.hover(); await player.getByRole('button', { name: 'Subtitles (C)', exact: true }).click();
  await expect(page.getByText('Timing: +0.5s (later)', { exact: true })).toBeVisible();
  await page.keyboard.press('Escape');
  await player.hover(); await player.getByRole('button', { name: 'Quality', exact: true }).click();
  fail = true;
  await page.getByRole('menuitemradio', { name: /^720 kbps/ }).click();
  await expect(player.getByText('Adaptive admission unavailable', { exact: true })).toBeVisible();
  fail = false; await player.getByRole('button', { name: 'Retry playback', exact: true }).click();
  await expect(video).toBeVisible();
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(1);
  await player.hover(); await player.getByRole('button', { name: 'Forward 10 seconds (L)', exact: true }).click();
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(5);
  await player.getByRole('button', { name: 'Close player', exact: true }).click();
});

test('adaptive: stored manual quality, explicit Original and Automatic survive reload on phone', async ({ page, movie }, info) => {
  await page.addInitScript(() => { if (localStorage.getItem('arc-quality') === null) localStorage.setItem('arc-quality', '2000'); });
  const requests: { max_kbps?: number }[] = [];
  page.on('request', r => { if (r.url().endsWith('/playback') && r.method() === 'POST') requests.push(r.postDataJSON()); });
  const { player } = await openPlayer(page, movie);
  expect(requests[0].max_kbps).toBe(2000);
  await page.setViewportSize({ width: 390, height: 844 });
  await player.hover(); await player.getByRole('button', { name: 'Quality', exact: true }).click();
  await expect(page.getByRole('menuitemradio', { name: /^2 Mbps/ })).toBeChecked();
  await page.getByRole('menuitemradio', { name: 'Original', exact: true }).click();
  await expect.poll(() => page.evaluate(() => localStorage.getItem('arc-quality'))).toBe('0');
  await expect.poll(() => requests.length).toBe(2);
  await expect.poll(() => page.locator('video').evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(0.5);
  await player.getByRole('button', { name: 'Close player', exact: true }).click();
  await page.reload();
  await openPlayer(page, movie);
  expect(requests.at(-1)?.max_kbps).toBeUndefined();
  await player.hover(); await player.getByRole('button', { name: 'Quality', exact: true }).click();
  await expect(page.getByRole('menuitemradio', { name: 'Original', exact: true })).toBeChecked();
  await page.getByRole('menuitemradio', { name: 'Automatic', exact: true }).click();
  expect(await page.evaluate(() => localStorage.getItem('arc-quality'))).toBe('auto');
  await player.hover(); await player.getByRole('button', { name: 'Quality', exact: true }).click();
  await expect(page.getByRole('menuitemradio', { name: 'Automatic', exact: true })).toBeChecked();
  await page.screenshot({ path: info.outputPath('automatic-quality-phone.png') });
  await page.keyboard.press('Escape');
  await player.getByRole('button', { name: 'Close player', exact: true }).click();
});
