import { test, expect, type Page } from '@playwright/test';
const password = 'Arc-browser-test-123';

test.beforeEach(async ({ page }) => {
  const r = await fetch('http://127.0.0.1:18200/reset', { method: 'POST' });
  if (!r.ok) throw new Error(await r.text());
  await r.json();
  await page.context().route('**/*', route => { const u = new URL(route.request().url()); return u.hostname === '127.0.0.1' || u.protocol === 'data:' ? route.continue() : route.abort(); });
  await setup(page);
});
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

test('hardware detection explains unavailable encoding and offers recoverable Retest', async ({ page }) => {
  let checking = false;
  let available = false;
  await page.route('**/api/settings', async route => {
    if (route.request().method() !== 'GET') return route.continue();
    const response = await route.fetch();
    const settings = await response.json();
    await route.fulfill({ response, json: { ...settings, transcoder: { checked: true, checking, tone_mapping: true,
      hw_encoder: available ? 'h264_vaapi' : '', hw_label: available ? 'VA-API' : '',
      attempts: [{ encoder: 'h264_vaapi', device: '/dev/dri/renderD129', error: available ? '' : 'Driver initialization failed' }] } } });
  });
  let fail = true;
  await page.route('**/api/transcoder/detect', async route => {
    if (fail) return route.fulfill({ status: 503, json: { error: 'Detection temporarily unavailable' } });
    checking = true;
    return route.fulfill({ status: 202, json: { started: true } });
  });
  await page.goto('/settings?section=playback');
  const retest = page.getByRole('button', { name: 'Retest hardware encoding', exact: true });
  await expect(page.getByRole('switch', { name: 'Use hardware acceleration', exact: true })).toBeDisabled();
  await page.getByText('Hardware detection details', { exact: true }).click();
  await expect(page.getByText(/Driver initialization failed/)).toBeVisible();
  await retest.click();
  await expect(page.getByText('Detection temporarily unavailable', { exact: true })).toBeVisible();
  fail = false;
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(retest).toBeDisabled();
  checking = false; available = true;
  await expect(page.getByRole('switch', { name: 'Use hardware acceleration', exact: true })).toBeEnabled();
  await page.unroute('**/api/transcoder/detect');
  const viewer = await (await page.request.post('/api/profiles', { data: { name: 'Hardware viewer', color: 'blue' } })).json();
  await page.request.post('/api/auth/profile', { data: { id: viewer.id } });
  expect((await page.request.post('/api/transcoder/detect')).status()).toBe(403);
});
