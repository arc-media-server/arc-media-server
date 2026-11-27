import { test, expect } from '@playwright/test';

test.beforeEach(async () => {
  const response = await fetch('http://127.0.0.1:18200/reset', { method: 'POST' });
  expect(response.ok).toBeTruthy();
});

test('security headers: production UI works while inline scripts and framing are blocked', async ({ page }) => {
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  const response = await page.goto('/');
  expect(response?.headers()['x-content-type-options']).toBe('nosniff');
  expect(response?.headers()['x-frame-options']).toBe('DENY');
  await expect(page.getByRole('heading', { name: 'Choose a database', exact: true })).toBeVisible();
  await page.evaluate(() => {
    const script = document.createElement('script');
    script.textContent = 'window.__arc_inline_marker = true';
    document.head.appendChild(script);
  });
  expect(await page.evaluate(() => ('__arc_inline_marker' in window))).toBe(false);
  await page.route('http://127.0.0.1:18200/frame-harness', route => route.fulfill({
    contentType: 'text/html', body: '<html><body></body></html>',
  }));
  await page.goto('http://127.0.0.1:18200/frame-harness');
  await page.evaluate(() => new Promise<void>(resolve => {
    const frame = document.createElement('iframe');
    frame.onload = () => resolve();
    frame.src = 'http://127.0.0.1:18199/';
    document.body.appendChild(frame);
  }));
  await expect(page.frameLocator('iframe').locator('#root')).toHaveCount(0);
  expect(errors).toEqual([]);
});

test('security headers: API documentation renders under the production content policy', async ({ page }) => {
  const response = await page.goto('/docs/');
  expect(response?.headers()['content-security-policy']).toContain("script-src 'self'");
  await expect(page.getByText('Arc Media Server API', { exact: true }).first()).toBeVisible();
});
