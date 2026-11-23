import { test, expect } from '@playwright/test';
import { readFile, mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';

test('dashboard opens device logs and downloads the selected log', async ({ page }) => {
  expect((await fetch('http://127.0.0.1:18200/reset', { method: 'POST' })).ok).toBe(true);
  const password = 'Arc-client-log-fixture';
  expect((await page.request.post('/api/auth/setup', { data: { name: 'Admin', password, local_bypass: false } })).ok()).toBe(true);
  const login = await page.request.post('/api/v1/auth/login', { data: { password, device: 'Living room Roku' } });
  expect(login.ok()).toBe(true);
  const { token } = await login.json();
  const status = await page.request.get('/api/v1/auth/status');
  expect(status.ok()).toBe(true);
  const { profiles } = await status.json();
  const headers = { Authorization: `Bearer ${token}` };
  expect((await page.request.post('/api/v1/auth/profile', { headers, data: { id: profiles[0].id } })).ok()).toBe(true);

  await page.goto('/dashboard');
  await page.getByRole('group', { name: 'Log views' }).getByRole('button', { name: 'Devices' }).click();
  await expect(page.getByText('No devices have sent logs recently.')).toBeVisible();
  const data = { platform: 'roku', entries: [{ time_ms: Date.now(), level: 'error', event: 'playback_failed' }] };
  expect((await page.request.post('/api/v1/client-logs', { headers, data })).status()).toBe(204);
  await page.getByRole('button', { name: 'Refresh logs' }).click();
  await expect(page.getByText('ERROR playback_failed')).toBeVisible();
  const downloaded = page.waitForEvent('download');
  await page.getByRole('button', { name: 'Download logs' }).click();
  const file = await downloaded;
  expect(file.suggestedFilename()).toBe('arc-device-logs.txt');
  expect(await readFile(await file.path(), 'utf8')).toContain('ERROR playback_failed');
  if (process.env.ARC_SCREENSHOT_DIR) {
    await mkdir(process.env.ARC_SCREENSHOT_DIR, { recursive: true });
    await page.locator('section').filter({ has: page.getByRole('heading', { name: 'Device logs' }) })
      .screenshot({ path: resolve(process.env.ARC_SCREENSHOT_DIR, 'device-logs.jpg'), type: 'jpeg' });
  }
});
