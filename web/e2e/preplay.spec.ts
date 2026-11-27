import { test as base, expect, type Page } from '@playwright/test';

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

test('preplay: explicit audio/subtitles survive the first request and optimized eligibility', async ({ page, media }, info) => {
  const [item] = await addLibrary(page, media.audio);
  expect((await page.request.post('/api/optimize', { data: { item_ids: [item.id], quality: 720 } })).ok()).toBeTruthy();
  await expect.poll(async () => (await (await page.request.get('/api/optimized')).json()).versions[0]?.status, { timeout: 30_000 }).toBe('done');
  await page.goto(`/movie/${item.id}`);
  await focusedPlayback(page);
  let playbackRequests = 0;
  let preferenceWrites = 0;
  page.on('request', request => {
    if (request.url().endsWith('/playback')) playbackRequests++;
    if (request.url().endsWith('/api/me/prefs') && request.method() === 'PATCH') preferenceWrites++;
  });
  await page.getByRole('button', { name: 'Playback options', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'Playback options', exact: true });
  await expect(dialog.getByRole('combobox', { name: 'Audio', exact: true })).toBeEnabled();
  await expect(dialog.getByRole('combobox', { name: 'Audio', exact: true }).getByRole('option', { name: /^Automatic — English/ })).toHaveCount(1);
  await dialog.getByRole('combobox', { name: 'Audio', exact: true }).selectOption('2');
  const french = await dialog.getByRole('combobox', { name: 'Subtitles', exact: true }).getByRole('option', { name: /^French/ }).getAttribute('value');
  expect(french).toBeTruthy();
  await dialog.getByRole('combobox', { name: 'Subtitles', exact: true }).selectOption(french!);
  expect(playbackRequests).toBe(0);
  expect(preferenceWrites).toBe(0);
  await page.setViewportSize({ width: 390, height: 900 });
  await page.screenshot({ path: info.outputPath('preplay-phone.png') });
  let refuseFirst = true;
  const attempted: { audio_stream?: number; file_id?: number }[] = [];
  await page.route('**/playback', route => {
    attempted.push(route.request().postDataJSON());
    if (!refuseFirst) return route.continue();
    refuseFirst = false;
    return route.fulfill({ status: 503, json: { error: 'Preplay start fixture unavailable' } });
  });
  const chosenFile = Number(await dialog.getByRole('combobox', { name: 'Version', exact: true }).inputValue());
  const response = page.waitForResponse(r => r.url().endsWith('/playback') && r.ok());
  await dialog.getByRole('button', { name: 'Play', exact: true }).click();
  await expect(page.getByText('Preplay start fixture unavailable', { exact: true })).toBeVisible();
  expect(preferenceWrites).toBe(0);
  await page.getByRole('button', { name: 'Retry playback', exact: true }).click();
  const result = await response;
  expect(attempted).toHaveLength(2);
  for (const attempt of attempted) expect(attempt).toMatchObject({ audio_stream: 2, file_id: chosenFile });
  expect(result.request().postDataJSON()).toMatchObject({ audio_stream: 2, file_id: chosenFile });
  const plan = await result.json();
  expect(plan.audio_stream).toBe(2);
  expect(plan.optimized).toBeUndefined();
  await expect(page.locator('[data-subtitles]')).toHaveText('Preplay French subtitle');
  await expect.poll(() => page.locator('video').evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(1);
  await expect.poll(async () => {
    const p = await (await page.request.get('/api/me/prefs')).json();
    return [p.audio_language, p.subtitle_language];
  }).toEqual(['fra', 'fra']);
  await closePlayer(page);
  await page.getByRole('button', { name: 'Playback options', exact: true }).click();
  await dialog.getByRole('combobox', { name: 'Audio', exact: true }).selectOption('1');
  await dialog.getByRole('combobox', { name: 'Subtitles', exact: true }).selectOption('off');
  const englishResponse = page.waitForResponse(r => r.url().endsWith('/playback') && r.ok());
  await dialog.getByRole('button', { name: /^(Play|Resume)$/, exact: true }).click();
  const english = await (await englishResponse).json();
  expect(english.optimized).toMatchObject({ source_audio_stream: 1, audio: { language: 'eng', codec: 'aac', channels: 2 } });
  await expect.poll(() => page.locator('video').evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(1);
  await expect(page.locator('[data-subtitles]')).toHaveCount(0);
  await expect.poll(async () => (await (await page.request.get('/api/me/prefs')).json()).audio_language).toBe('eng');
  await closePlayer(page);
});

test('preplay: separate editions link across pages and version changes reset track IDs', async ({ page, media }, info) => {
  const items = await addLibrary(page, media.preplay);
  expect(items).toHaveLength(2);
  const theatrical = items.find((item: { edition?: string }) => item.edition === 'Theatrical Cut');
  const directors = items.find((item: { edition?: string }) => item.edition === "Director's Cut");
  expect(theatrical).toBeTruthy();
  expect(directors).toBeTruthy();
  const detail = await (await page.request.get(`/api/items/${theatrical.id}`)).json();
  expect(detail.files).toHaveLength(2);
  const alternate = detail.files.find((f: { id: number; path: string }) => f.path.includes('alternate'));
  const first = detail.files.find((f: { id: number }) => f.id !== alternate.id);
  const directorFiles = (await (await page.request.get(`/api/items/${directors.id}`)).json()).files;
  expect(directorFiles).toHaveLength(1);
  await page.goto(`/library/${theatrical.library_id}`);
  await expect(page.getByText('2020 · Theatrical Cut', { exact: true })).toBeVisible();
  await expect(page.getByText("2020 · Director's Cut", { exact: true })).toBeVisible();
  await page.goto(`/movie/${theatrical.id}`);
  await focusedPlayback(page);
  let fail = true;
  let requests = 0;
  await page.route('**/playback-options', route => fail ? route.fulfill({ status: 503, json: { error: 'Options fixture unavailable' } }) : route.continue());
  page.on('request', r => { if (r.url().endsWith('/playback')) requests++; });
  const before = await (await page.request.get('/api/me/prefs')).json();
  await page.getByRole('button', { name: 'Playback options', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'Playback options', exact: true });
  await expect(dialog.getByRole('alert')).toContainText('Options fixture unavailable');
  await expect(dialog.getByRole('button', { name: 'Play', exact: true })).toBeDisabled();
  fail = false;
  await dialog.getByRole('button', { name: 'Retry', exact: true }).click();
  const version = dialog.getByRole('combobox', { name: 'Version', exact: true });
  await expect(version.getByRole('option')).toHaveCount(2);
  await version.selectOption(String(first.id));
  await dialog.getByRole('combobox', { name: 'Audio', exact: true }).selectOption('2');
  await dialog.getByRole('combobox', { name: 'Subtitles', exact: true }).selectOption('off');
  await dialog.getByRole('combobox', { name: 'Version', exact: true }).selectOption(String(alternate.id));
  await expect(dialog).toContainText("Audio Movie · Theatrical Cut");
  await expect(dialog.getByRole('combobox', { name: 'Audio', exact: true })).toHaveValue('');
  await expect(dialog.getByRole('combobox', { name: 'Subtitles', exact: true })).toHaveValue('auto');
  await expect(dialog.getByRole('combobox', { name: 'Audio', exact: true }).getByRole('option', { name: /^Automatic — French/ })).toHaveCount(1);
  await page.screenshot({ path: info.outputPath('preplay-version.png') });
  await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
  expect(requests).toBe(0);
  expect(await (await page.request.get('/api/me/prefs')).json()).toEqual(before);
  await page.getByRole('link', { name: "Director's Cut", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`/movie/${directors.id}$`));
  await page.getByRole('button', { name: 'Playback options', exact: true }).click();
  await expect(dialog.getByRole('combobox', { name: 'Version', exact: true })).toHaveValue(String(directorFiles[0].id));
  await dialog.getByRole('combobox', { name: 'Audio', exact: true }).selectOption('1');
  const response = page.waitForResponse(r => r.url().endsWith('/playback') && r.ok());
  await dialog.getByRole('button', { name: 'Play', exact: true }).click();
  const result = await response;
  expect(result.request().postDataJSON()).toMatchObject({ file_id: directorFiles[0].id, audio_stream: 1 });
  expect((await result.json()).audio_tracks.find((t: { index: number }) => t.index === 1).language).toBe('fra');
  await expect.poll(() => page.locator('video').evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(1);
  await closePlayer(page);
});

test('preplay: episode header and keyboard row choices target the selected episode', async ({ page, media }, info) => {
  await addLibrary(page, media.shows, true);
  const lib = (await (await page.request.get('/api/libraries')).json())[0];
  const series = (await (await page.request.get(`/api/items?library_id=${lib.id}&kind=series`)).json()).items[0];
  await page.goto(`/series/${series.id}`);
  await focusedPlayback(page);
  await page.getByRole('button', { name: 'Playback options', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'Playback options', exact: true });
  await expect(dialog).toContainText('First Chapter');
  await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
  // An episode's playback options are in its More menu; choosing them by keyboard doesn't start playback.
  await page.getByRole('button', { name: 'More for First Chapter', exact: true }).focus();
  await page.keyboard.press('Enter');
  await page.getByRole('menuitem', { name: 'Playback options…', exact: true }).focus();
  await page.keyboard.press('Enter');
  const subtitle = dialog.getByRole('combobox', { name: 'Subtitles', exact: true });
  await expect(subtitle).toBeEnabled();
  const french = await subtitle.getByRole('option', { name: /^French/ }).getAttribute('value');
  await subtitle.selectOption(french!);
  await expect(page.locator('video')).toHaveCount(0);
  await page.screenshot({ path: info.outputPath('episode-preplay.png') });
  await dialog.getByRole('button', { name: 'Play', exact: true }).click();
  await expect(page.locator('[data-subtitles]')).toHaveText('Episode French subtitle');
  await expect.poll(() => page.locator('video').evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(1);
  await closePlayer(page);
});

test('preplay: picture subtitles enter the first request and unavailable subtitles explain fallback', async ({ page, media }) => {
  const items = await addLibrary(page, media.movies);
  const item = items.find((i: { title: string }) => i.title === 'Arc Movie');
  await page.route('**/playback-options', async route => {
    const response = await route.fetch();
    const data = await response.json();
    data.subtitle_tracks.push({ id: 's:99', language: 'jpn', codec: 'hdmv_pgs_subtitle', image: true, external: false, default: false, forced: false, hearing_impaired: false });
    return route.fulfill({ response, json: data });
  });
  await page.goto(`/movie/${item.id}`);
  await page.getByRole('button', { name: 'Playback options', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'Playback options', exact: true });
  await dialog.getByRole('combobox', { name: 'Subtitles', exact: true }).selectOption('s:99');
  await expect(dialog.getByText('Picture subtitles require video conversion.', { exact: true })).toBeVisible();
  let captured: { burn_subtitle?: string } | undefined;
  await page.route('**/playback', route => {
    captured = route.request().postDataJSON();
    return route.fulfill({ status: 503, json: { error: 'Picture request fixture: conversion unavailable' } });
  });
  await dialog.getByRole('button', { name: 'Play', exact: true }).click();
  await expect(page.getByText('Picture request fixture: conversion unavailable', { exact: true })).toBeVisible();
  expect(captured?.burn_subtitle).toBe('s:99');
  await closePlayer(page);
  await page.unroute('**/playback');
  await page.getByRole('button', { name: 'Playback options', exact: true }).click();
  const french = await dialog.getByRole('combobox', { name: 'Subtitles', exact: true }).getByRole('option', { name: /^French/ }).getAttribute('value');
  await dialog.getByRole('combobox', { name: 'Subtitles', exact: true }).selectOption(french!);
  await page.route('**/playback', async route => {
    const response = await route.fetch();
    const data = await response.json();
    data.subtitle_tracks = data.subtitle_tracks.filter((t: { id: string }) => t.id !== french);
    return route.fulfill({ response, json: data });
  });
  await dialog.getByRole('button', { name: /^(Play|Resume)$/, exact: true }).click();
  await expect(page.getByRole('region', { name: 'Media player', exact: true }).getByRole('status')).toContainText('The selected subtitles are no longer available. Using your profile defaults.');
  await page.getByRole('button', { name: 'Subtitles (C)', exact: true }).click();
  await page.getByRole('menuitem', { name: /^English/ }).click();
  await expect(page.locator('[data-subtitles]')).toHaveText('English fixture subtitle');
  await expect(page.getByText('The selected subtitles are no longer available. Using your profile defaults.', { exact: true })).toHaveCount(0);
  await closePlayer(page);
});
