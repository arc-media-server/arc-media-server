import { test, expect } from '@playwright/test';

// Any profile sets, changes and removes its own PIN in Settings; changing one needs the current PIN.
test('own PIN: set it, it is asked for on switching, change needs the current one, then remove it', async ({ page }, info) => {
  const reset = await fetch('http://127.0.0.1:18200/reset', { method: 'POST' });
  expect(reset.ok).toBeTruthy();
  expect((await page.request.post('/api/auth/setup', { data: { name: 'Pin owner', password: 'Own-pin-fixture-123', local_bypass: false, tmdb_api_key: 'fixture-not-a-secret' } })).ok()).toBeTruthy();
  expect((await page.request.patch('/api/settings', { data: { scan_automatic: false, scan_on_start: false, scan_interval_minutes: 0 } })).ok()).toBeTruthy();
  const kid = await (await page.request.post('/api/profiles', { data: { name: 'Sam', color: 'teal' } })).json();
  expect((await page.request.post('/api/auth/profile', { data: { id: kid.id } })).ok()).toBeTruthy();
  await page.emulateMedia({ colorScheme: 'dark' });
  await page.setViewportSize({ width: 1280, height: 800 });
  await page.goto('/settings');
  const section = page.locator('section', { has: page.getByRole('heading', { name: 'PIN', exact: true }) }).first();
  await page.getByRole('button', { name: 'Set a PIN', exact: true }).click();
  await page.getByRole('textbox', { name: 'New PIN' }).fill('1234');
  await section.scrollIntoViewIfNeeded();
  await page.screenshot({ path: info.outputPath('pin-set.png') });
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByText('PIN is on', { exact: true })).toBeVisible();
  await page.screenshot({ path: info.outputPath('pin-on.png') });

  // Switching to Sam now asks for the PIN.
  expect((await page.request.post('/api/auth/profile', { data: { id: kid.id } })).status()).toBe(401);
  expect((await page.request.post('/api/auth/profile', { data: { id: kid.id, pin: '1234' } })).ok()).toBeTruthy();

  // Changing it needs the current PIN.
  await page.getByRole('button', { name: 'Change or remove', exact: true }).click();
  await page.getByRole('textbox', { name: 'Current PIN' }).fill('0000');
  await page.getByRole('textbox', { name: 'New PIN' }).fill('5678');
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText("current PIN isn't right");
  await page.screenshot({ path: info.outputPath('pin-wrong.png') });
  await page.getByRole('textbox', { name: 'Current PIN' }).fill('1234');
  await page.getByRole('button', { name: 'Remove PIN', exact: true }).click();
  await expect(page.getByText('No PIN', { exact: true })).toBeVisible();
  expect((await page.request.post('/api/auth/profile', { data: { id: kid.id } })).ok()).toBeTruthy();
});
