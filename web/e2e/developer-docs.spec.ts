import { test, expect, type Page } from '@playwright/test';
import { readFileSync } from 'node:fs';

const password = 'Documentation-fixture-password';
const guide = readFileSync(new URL('../../docs/CLIENT-INTEGRATION.md', import.meta.url), 'utf8');
let generatedMovies: string;
let generatedShows: string;
function example(name: string) {
  const marker = `<!-- example: ${name} -->`;
  const start = guide.indexOf(marker);
  if (start < 0) throw new Error(`Missing documentation example ${name}`);
  const json = guide.slice(start + marker.length).match(/```json\s*\n([\s\S]*?)\n```/);
  if (!json) throw new Error(`Missing JSON for ${name}`);
  return JSON.parse(json[1]);
}

test.beforeEach(async ({ page }) => {
  const reset = await fetch('http://127.0.0.1:18200/reset', { method: 'POST' });
  expect(reset.ok).toBeTruthy();
  const media = await reset.json();
  generatedMovies = media.movies;
  generatedShows = media.shows;
  await page.context().route('**/*', route => {
    const url = new URL(route.request().url());
    return url.hostname === '127.0.0.1' || url.protocol === 'data:' ? route.continue() : route.abort();
  });
});

test('developer docs: the served guide link opens the bundled canonical walkthrough', async ({ page }, info) => {
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.goto('/docs/');
  await page.getByRole('link', { name: 'Client integration guide', exact: true }).click();
  await expect(page).toHaveURL(/\/docs\/client-integration\.html$/);
  await expect(page.getByRole('heading', { name: 'Integrating an Arc client', exact: true })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Get a device token', exact: true })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Preview, play, report progress and stop', exact: true })).toHaveCount(1);
  await expect(page.getByRole('heading', { name: 'Browse episodes and alternate orders', exact: true })).toHaveCount(1);
  await expect(page.getByRole('heading', { name: 'Watch Together: enter a room, then run ordinary playback', exact: true })).toHaveCount(1);
  await expect(page.getByRole('heading', { name: 'Downloads: check support before showing offline controls', exact: true })).toHaveCount(1);
  await expect(page.getByRole('link', { name: 'original-download guide', exact: true })).toHaveAttribute('href', 'https://github.com/arc-media-server/arc-media-server/blob/main/docs/DOWNLOADS-OFFLINE-API.md');
  await page.getByRole('heading', { name: 'Browse episodes and alternate orders', exact: true }).scrollIntoViewIfNeeded();
  await page.screenshot({ path: info.outputPath('developer-guide-desktop.png') });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.getByRole('heading', { name: 'Get a device token', exact: true }).scrollIntoViewIfNeeded();
  await page.screenshot({ path: info.outputPath('developer-guide-phone.png') });
  await page.getByRole('heading', { name: 'Downloads: check support before showing offline controls', exact: true }).scrollIntoViewIfNeeded();
  await page.screenshot({ path: info.outputPath('developer-guide-downloads-phone.png') });
  await page.getByRole('link', { name: 'API.md', exact: true }).first().click();
  await expect(page).toHaveURL(/\/docs\/$/);
  await expect(page.getByText('Arc Media Server API', { exact: true }).first()).toBeVisible();
  expect(errors).toEqual([]);
});

test('developer docs: profile schemas accept nullable responses and minimal requests', async ({ page }) => {
  const setup = await page.request.post('/api/v1/auth/setup', { data: { name: 'Docs fixture', password, local_bypass: false } });
  expect(setup.status()).toBe(201);
  const initial = await setup.json();
  expect(initial.library_ids).toBeNull();
  expect(initial.exclude_labels).toBeNull();
  const schema = await (await page.request.get('/api/openapi.json')).json();
  const components = schema.components.schemas;
  expect(components.Profile.properties.library_ids.anyOf.map((s: { type: string }) => s.type)).toEqual(['array', 'null']);
  const create = await page.request.post('/api/v1/profiles', { data: { name: 'Minimal fixture' } });
  expect(create.status()).toBe(201);
  const profile = await create.json();
  expect(components.Profile.properties.exclude_labels.anyOf.map((s: { type: string }) => s.type)).toEqual(['array', 'null']);
  expect(components.InputCreateProfile.required).toEqual(['name']);
  expect(components.InputProfileBody.required ?? []).toEqual([]);
  expect((await page.request.patch(`/api/v1/profiles/${profile.id}`, { data: { color: 'teal' } })).status()).toBe(200);
  expect((await page.request.patch(`/api/v1/profiles/${profile.id}`, { data: { library_ids: [] } })).status()).toBe(200);
  let profiles = await (await page.request.get('/api/v1/profiles')).json();
  expect(profiles.find((p: { id: string }) => p.id === profile.id).library_ids).toEqual([]);
  expect((await page.request.patch(`/api/v1/profiles/${profile.id}`, { data: { library_ids: null } })).status()).toBe(200);
  profiles = await (await page.request.get('/api/v1/profiles')).json();
  expect(profiles.find((p: { id: string }) => p.id === profile.id).library_ids).toBeNull();
});

test('developer docs: canonical JSON authenticates, browses, previews and reports playback', async ({ page }) => {
  const setup = await page.request.post('/api/v1/auth/setup', { data: { name: 'Docs fixture', password, local_bypass: false, tmdb_api_key: 'fixture-key-not-a-secret' } });
  expect(setup.status()).toBe(201);
  const owner = await setup.json();
  const library = await page.request.post('/api/v1/libraries', { data: { name: 'Documentation library', kind: 'movies', paths: [generatedMovies] } });
  expect(library.status()).toBe(201);
  const lib = await library.json();
  await expect.poll(async () => (await (await page.request.get(`/api/v1/libraries/${lib.id}/scan`)).json()).running).toBe(false);
  const login = await page.request.post('/api/v1/auth/login', { data: { ...example('device-login'), password } });
  expect(login.status()).toBe(200);
  const token = (await login.json()).token;
  expect(typeof token).toBe('string');
  const headers = { Authorization: `Bearer ${token}` };
  const status = await (await page.request.get('/api/v1/auth/status', { headers })).json();
  expect(status.profile).toBeNull();
  expect((await page.request.post('/api/v1/auth/profile', { headers, data: { ...example('select-profile'), id: owner.id } })).status()).toBe(200);
  expect((await page.request.get('/api/v1/identity', { headers })).status()).toBe(200);
  const browse = await (await page.request.get(`/api/v1/items?library_id=${lib.id}&kind=movie&limit=20&offset=0&sort=added&reverse=1`, { headers })).json();
  expect(browse.items.length).toBeGreaterThan(0);
  const item = browse.items.find((i: { title: string }) => i.title === 'Arc Movie') ?? browse.items[0];
  const body = example('playback');
  const previewResponse = await page.request.post(`/api/v1/items/${item.id}/playback-preview`, { headers, data: body });
  expect(previewResponse.status()).toBe(200);
  const preview = await previewResponse.json();
  expect(preview.decision_only).toBe(true);
  expect(preview.can_start).toBe(true);
  expect(preview.url).toBe('');
  const start = await page.request.post(`/api/v1/items/${item.id}/playback`, { headers, data: body });
  expect(start.status()).toBe(200);
  const plan = await start.json();
  expect(typeof plan.url).toBe('string');
  expect(plan.url.length > 0).toBe(true);
  const progress = await page.request.put(`/api/v1/items/${item.id}/progress`, { headers, data: { ...example('progress'), position_ms: 1000, duration_ms: 0, state: 'stopped' } });
  expect(progress.status()).toBe(200);
  const saved = await progress.json();
  expect(saved.played).toBe(false);
  expect(saved.last_played_at).toBeGreaterThan(0);
  expect(saved.position_ms).toBeUndefined(); // Before the first minute, no resume point is kept.
  if (plan.session_id) expect((await page.request.delete(`/api/v1/transcode/${plan.session_id}`, { headers })).status()).toBe(204);
  const browserLogin = await page.request.post('/api/v1/auth/login', { data: { password } });
  expect(browserLogin.status()).toBe(204);
  expect((await browserLogin.body()).length).toBe(0);
});

async function taskLibrary(page: Page, shows = false) {
  const setup = await page.request.post('/api/v1/auth/setup', { data: {
    name: 'Task guide owner', password, local_bypass: false, tmdb_api_key: 'fixture-key-not-a-secret',
  } });
  expect(setup.status()).toBe(201);
  const owner = await setup.json();
  const created = await page.request.post('/api/v1/libraries', { data: {
    name: 'Task guide library', kind: shows ? 'shows' : 'movies',
    paths: [shows ? generatedShows : generatedMovies], source_ordering: 'tmdb_aired',
  } });
  expect(created.status()).toBe(201);
  const lib = await created.json();
  await expect.poll(async () => (await (await page.request.get(`/api/v1/libraries/${lib.id}/scan`)).json()).running).toBe(false);
  const items = (await (await page.request.get(`/api/v1/items?library_id=${lib.id}&kind=${shows ? 'series' : 'movie'}`)).json()).items;
  expect(items.length).toBeGreaterThan(0);
  return { owner, lib, item: items[0] };
}

async function deviceHeaders(page: Page, profile: string) {
  const login = await page.request.post('/api/v1/auth/login', { data: { ...example('device-login'), password } });
  expect(login.status()).toBe(200);
  const headers = { Authorization: `Bearer ${(await login.json()).token}` };
  expect((await page.request.post('/api/v1/auth/profile', { headers, data: { ...example('select-profile'), id: profile } })).status()).toBe(200);
  return headers;
}

test('developer docs: episode-order examples preview, save, reset and preserve episode identities', async ({ page }) => {
  const { item } = await taskLibrary(page, true);
  const base = `/api/v1/series/${item.id}`;
  const initial = await (await page.request.get(`${base}/orderings`)).json();
  const alternate = initial.orderings.find((o: { name: string }) => o.name === 'Story Order');
  expect(typeof alternate.id).toBe('number');
  expect(initial.display_choice_id).toBe(0);
  const original = await (await page.request.get(`${base}/seasons`)).json();
  const ids = (body: typeof original) => [...body.seasons.flatMap((s: { episodes: { id: string }[] }) => s.episodes.map(e => e.id)), ...body.unplaced.map((e: { id: string }) => e.id)].sort();
  const preview = await (await page.request.get(`${base}/seasons?ordering=${alternate.id}&missing=1`)).json();
  expect(preview.ordering.id).toBe(alternate.id);
  expect(ids(preview)).toEqual(ids(original));
  expect(preview.unplaced.length).toBeGreaterThan(0); // Generated on-disk episode outside provider ordering.
  expect((await (await page.request.get(`${base}/orderings`)).json()).display_choice_id).toBe(0);
  const saved = await page.request.put(`${base}/display-ordering`, { data: { ...example('display-ordering'), ordering_id: alternate.id } });
  expect(saved.status()).toBe(200);
  expect((await saved.json()).display_choice_id).toBe(alternate.id);
  const arranged = await (await page.request.get(`${base}/seasons`)).json();
  expect(arranged.ordering.id).toBe(alternate.id);
  expect(ids(arranged)).toEqual(ids(original));
  const episodes = arranged.seasons.flatMap((s: { episodes: { id: string; available: boolean }[] }) => s.episodes).filter((e: { available: boolean }) => e.available);
  expect(episodes.length).toBe(2);
  const next = await page.request.get(`/api/v1/items/${episodes[0].id}/next`);
  expect(next.status()).toBe(200);
  expect((await next.json()).id).toBe(episodes[1].id);
  expect((await page.request.get(`/api/v1/items/${episodes[1].id}/next`)).status()).toBe(204);
  const reset = await page.request.put(`${base}/display-ordering`, { data: example('display-ordering-reset') });
  expect(reset.status()).toBe(200);
  expect((await reset.json()).display_choice_id).toBe(0);
  expect(ids(await (await page.request.get(`${base}/seasons`)).json())).toEqual(ids(original));
  const viewer = await (await page.request.post('/api/v1/profiles', { data: { name: 'Guide reader' } })).json();
  const headers = await deviceHeaders(page, viewer.id);
  expect((await page.request.get(`${base}/seasons?ordering=${alternate.id}`, { headers })).status()).toBe(200);
  expect((await page.request.put(`${base}/display-ordering`, { headers, data: example('display-ordering-reset') })).status()).toBe(403);
});

test('developer docs: Watch Together examples use device ownership, revisions and capability gates', async ({ page }) => {
  const { owner, item } = await taskLibrary(page);
  const host = await deviceHeaders(page, owner.id);
  const guest = await deviceHeaders(page, owner.id); // Same profile, a separate device session.
  const caps = await (await page.request.get('/api/v1/capabilities', { headers: host })).json();
  expect(caps.features.watch_together).toEqual({ supported: true, available: true, allowed: true });
  // Original downloads are merged (#62): an administrator is allowed them on this server.
  for (const feature of ['downloads_original', 'downloads_preparation']) {
    expect(caps.features[feature]).toMatchObject({ supported: true, allowed: true });
    expect(typeof caps.features[feature].available).toBe('boolean');
  }
  const created = await page.request.post('/api/v1/watch-together', { headers: host, data: { ...example('watch-room'), item_id: item.id } });
  expect(created.status()).toBe(201);
  const invitation = await created.json();
  const recovered = await page.request.get('/api/v1/watch-together/current', { headers: host });
  expect(recovered.status()).toBe(200);
  expect((await recovered.json()).id).toBe(invitation.room.id);
  const joined = await page.request.post('/api/v1/watch-together/join', { headers: guest, data: { ...example('watch-join'), code: invitation.invite_code } });
  expect(joined.status()).toBe(200);
  const room = await joined.json();
  expect(room.participants.length).toBe(2);
  expect(room.self_id).not.toBe(invitation.room.self_id);
  const base = `/api/v1/watch-together/${room.id}`;
  const stale = await page.request.post(`${base}/commands`, { headers: host, data: { ...example('watch-command'), revision: invitation.room.revision } });
  expect(stale.status()).toBe(409);
  const current = (await stale.json()).room;
  const pause = await page.request.post(`${base}/commands`, { headers: host, data: { ...example('watch-command'), revision: current.revision } });
  expect(pause.status()).toBe(200);
  expect((await pause.json()).state).toBe('paused');
  expect((await page.request.post(`${base}/leave`, { headers: guest })).status()).toBe(204);
  expect((await page.request.get('/api/v1/watch-together/current', { headers: guest })).status()).toBe(404);
  expect((await page.request.post(`${base}/leave`, { headers: host })).status()).toBe(204);
});

test('developer docs: TV linking approves once and selects a profile on the issued token', async ({ page, request }) => {
  const setup = await page.request.post('/api/v1/auth/setup', { data: { name: 'Link guide owner', password, local_bypass: false } });
  expect(setup.status()).toBe(201);
  const owner = await setup.json();
  const started = await request.post('/api/v1/auth/link', { data: example('device-link') });
  expect(started.status()).toBe(200);
  const pending = await started.json();
  expect(pending.expires_in).toBe(600);
  expect((await request.get(`/api/v1/auth/link/${pending.poll}`)).status()).toBe(202);
  expect((await page.request.post('/api/v1/auth/link/approve', { data: { code: pending.code } })).status()).toBe(200);
  const collected = await request.get(`/api/v1/auth/link/${pending.poll}`);
  expect(collected.status()).toBe(200);
  const headers = { Authorization: `Bearer ${(await collected.json()).token}` };
  expect((await request.get(`/api/v1/auth/link/${pending.poll}`)).status()).toBe(404);
  expect((await (await request.get('/api/v1/auth/status', { headers })).json()).profile).toBeNull();
  expect((await request.post('/api/v1/auth/profile', { headers, data: { ...example('select-profile'), id: owner.id } })).status()).toBe(200);
  expect((await request.get('/api/v1/identity', { headers })).status()).toBe(200);
});
