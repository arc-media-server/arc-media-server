import { test, expect } from '@playwright/test';
import { mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';

test('Tool tokens Settings creates once, copies, lists and confirms revocation', async ({ page, context }) => {
  expect((await fetch('http://127.0.0.1:18200/reset', { method: 'POST' })).ok).toBe(true);
  await context.route('**/*', route => {
    const url = new URL(route.request().url());
    return url.hostname === '127.0.0.1' || url.protocol === 'data:' ? route.continue() : route.abort();
  });
  const errors: string[] = [];
  page.on('pageerror', err => errors.push(err.message));
  expect((await page.request.post('/api/auth/setup', { data: { name: 'Admin', password: 'Arc-key-fixture-123', local_bypass: false } })).ok()).toBe(true);
  await page.emulateMedia({ colorScheme: 'dark' });
  await page.goto('/settings?section=access');
  const section = page.locator('section').filter({ has: page.getByRole('heading', { name: 'Devices', exact: true }) });
  await section.getByRole('button', { name: 'New token for a tool', exact: true }).click();
  await section.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(section.getByRole('textbox', { name: 'Tool name', exact: true })).toHaveCount(0);
  await section.getByRole('button', { name: 'New token for a tool', exact: true }).click();
  await section.getByRole('textbox', { name: 'Tool name', exact: true }).fill('Library automation');
  await section.getByRole('button', { name: 'Create token', exact: true }).click();
  const secretInput = section.getByRole('textbox', { name: 'New tool token', exact: true });
  await expect(secretInput).toBeVisible();
  const secret = await secretInput.inputValue();
  expect(secret.length > 40).toBe(true);
  await context.grantPermissions(['clipboard-read', 'clipboard-write']);
  await section.getByRole('button', { name: 'Copy', exact: true }).click();
  await expect(section.getByRole('button', { name: 'Copied', exact: true })).toBeVisible();
  expect(await page.evaluate(async value => await navigator.clipboard.readText() === value, secret)).toBe(true);
  await page.evaluate(() => navigator.clipboard.writeText(''));
  // HTTP-only browser contexts may not provide the Clipboard API at all.
  await page.evaluate(() => Object.defineProperty(navigator, 'clipboard', { configurable: true, value: undefined }));
  await section.getByRole('button', { name: 'Copied', exact: true }).click();
  await expect(section.getByRole('alert')).toHaveText('Could not copy. Select the token and copy it manually.');
  await page.evaluate(() => Reflect.deleteProperty(navigator, 'clipboard'));
  const headers = { Authorization: `Bearer ${secret}` };
  expect((await page.request.get('/api/v1/identity', { headers })).status()).toBe(200);
  const listed = await (await page.request.get('/api/v1/devices')).json();
  const tool = listed.find((d: { device: string; is_tool: boolean }) => d.device === 'Library automation' && d.is_tool);
  expect(!!tool).toBe(true);
  expect(JSON.stringify(listed).includes(secret)).toBe(false);
  await section.getByRole('button', { name: 'Done', exact: true }).click();
  await expect(secretInput).toHaveCount(0);
  await page.reload();
  await expect(secretInput).toHaveCount(0);
  await expect(section.getByRole('group', { name: 'Device: Library automation', exact: true })).toBeVisible();
  // Keep the change-note screenshot free of credentials. Use only synthetic fixture metadata.
  if (process.env.ARC_SCREENSHOT_DIR) {
    await page.setViewportSize({ width: 1440, height: 1000 });
    await section.scrollIntoViewIfNeeded();
    await mkdir(process.env.ARC_SCREENSHOT_DIR, { recursive: true });
    await section.screenshot({ path: resolve(process.env.ARC_SCREENSHOT_DIR, 'tool-tokens.jpg'), type: 'jpeg', quality: 85, animations: 'disabled' });
  }
  await page.setViewportSize({ width: 390, height: 844 });
  await section.getByRole('button', { name: 'Revoke', exact: true }).click();
  const confirmation = page.getByRole('alertdialog', { name: 'Revoke tool token?', exact: true });
  await confirmation.getByRole('button', { name: 'Cancel', exact: true }).click();
  expect((await page.request.get('/api/v1/identity', { headers })).status()).toBe(200);
  await section.getByRole('button', { name: 'Revoke', exact: true }).click();
  await confirmation.getByRole('button', { name: 'Revoke', exact: true }).click();
  await expect(section.getByRole('group', { name: 'Device: Library automation', exact: true })).toHaveCount(0);
  expect((await page.request.get('/api/v1/identity', { headers })).status()).toBe(401);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  // Non-admin profiles see neither tool-token controls nor management navigation.
  const viewer = await (await page.request.post('/api/profiles', { data: { name: 'Viewer', color: 'blue' } })).json();
  expect((await page.request.post('/api/auth/profile', { data: { id: viewer.id } })).ok()).toBe(true);
  await page.goto('/settings?section=access');
  await expect(page.getByRole('button', { name: 'New token for a tool', exact: true })).toHaveCount(0);
  expect((await page.request.get('/api/v1/devices')).status()).toBe(403);
  expect(errors).toEqual([]);
});
