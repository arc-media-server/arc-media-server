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
  await expect(page.getByRole('heading', { name: 'Welcome', exact: true })).toBeVisible();
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

test('hardware detection re-checks on its own and offers a graphics card choice', async ({ page }) => {
  let checking = false;
  let available = false;
  const devices = [
    { id: 'h264_qsv:/dev/dri/renderD128', label: 'Intel Quick Sync · renderD128', encoder: 'h264_qsv', device: '/dev/dri/renderD128' },
    { id: 'h264_vaapi:/dev/dri/renderD129', label: 'AMD (VA-API) · renderD129', encoder: 'h264_vaapi', device: '/dev/dri/renderD129' },
  ];
  await page.route('**/api/settings', async route => {
    const response = await route.fetch();
    const settings = await response.json();
    await route.fulfill({ response, json: { ...settings, transcoder: { checked: true, checking, tone_mapping: true,
      hw_encoder: available ? 'h264_qsv' : '', hw_label: available ? 'Intel Quick Sync' : '',
      devices: available ? devices : [],
      attempts: [{ encoder: 'h264_vaapi', device: '/dev/dri/renderD129', error: available ? '' : 'Driver initialization failed' }] } } });
  });
  let fail = true;
  let detections = 0;
  await page.route('**/api/transcoder/detect', async route => {
    detections++;
    if (fail) return route.fulfill({ status: 503, json: { error: 'Detection temporarily unavailable' } });
    checking = true;
    return route.fulfill({ status: 202, json: { started: true } });
  });
  await page.goto('/settings?section=playback');
  await expect(page.getByRole('switch', { name: 'Use hardware acceleration', exact: true })).toBeDisabled();
  await expect(page.getByText('No graphics card that can convert video was found, so Arc uses the processor.', { exact: true })).toBeVisible();
  // Nothing found: Settings checks again by itself, once; no Retest button or detection details.
  await expect(page.getByText('Detection temporarily unavailable', { exact: true })).toBeVisible();
  expect(detections).toBe(1);
  await expect(page.getByRole('button', { name: /Retest/ })).toHaveCount(0);
  await expect(page.getByText('Hardware detection details')).toHaveCount(0);
  fail = false;
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  checking = false; available = true;
  await expect(page.getByRole('switch', { name: 'Use hardware acceleration', exact: true })).toBeEnabled();
  const card = page.getByRole('combobox', { name: 'Graphics card', exact: true });
  await expect(card).toHaveValue('');
  await expect(card.locator('option')).toHaveText(['Auto', 'Intel Quick Sync · renderD128', 'AMD (VA-API) · renderD129']);
  await card.selectOption('h264_vaapi:/dev/dri/renderD129');
  await expect(card).toHaveValue('h264_vaapi:/dev/dri/renderD129');
  expect((await (await page.request.get('/api/settings')).json()).hw_device).toBe('h264_vaapi:/dev/dri/renderD129');
  await page.unroute('**/api/transcoder/detect');
  const viewer = await (await page.request.post('/api/profiles', { data: { name: 'Hardware viewer', color: 'blue' } })).json();
  await page.request.post('/api/auth/profile', { data: { id: viewer.id } });
  expect((await page.request.post('/api/transcoder/detect')).status()).toBe(403);
});
