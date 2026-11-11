import { test, expect } from '@playwright/test';
import { readFileSync } from 'node:fs';

const password = 'Documentation-fixture-password';
const guide = readFileSync(new URL('../../docs/CLIENT-INTEGRATION.md', import.meta.url), 'utf8');
let generatedMovies: string;
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
  generatedMovies = (await reset.json()).movies;
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
  await page.screenshot({ path: info.outputPath('developer-guide-desktop.png') });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.getByRole('heading', { name: 'Get a device token', exact: true }).scrollIntoViewIfNeeded();
  await page.screenshot({ path: info.outputPath('developer-guide-phone.png') });
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
