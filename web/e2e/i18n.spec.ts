import { test, expect } from '@playwright/test';

for (const locale of ['en-US', 'fr-FR']) {
  test.describe(`bundled English with ${locale} browser`, () => {
    test.use({ locale });
    test('first paint and profile interpolation retain English without nonexistent locale requests', async ({ page }) => {
      expect((await fetch('http://127.0.0.1:18200/reset', { method: 'POST' })).ok).toBe(true);
      const errors: string[] = [];
      const localeRequests: string[] = [];
      page.on('pageerror', error => errors.push(error.message));
      page.on('request', request => { if (/locales|\/fr[/-]|\/en[/-]|auth-[\w-]+\.js|common-[\w-]+\.js/.test(new URL(request.url()).pathname)) localeRequests.push(request.url()); });
      await page.context().route('**/*', route => new URL(route.request().url()).hostname === '127.0.0.1' ? route.continue() : route.abort());
      await page.addInitScript(() => {
        const headings: string[] = [];
        Object.assign(window, { firstHeadings: headings });
        new MutationObserver(() => {
          const text = document.querySelector('h1')?.textContent;
          if (text && headings.at(-1) !== text) headings.push(text);
        }).observe(document, { childList: true, subtree: true, characterData: true });
      });
      await page.goto('/');
      await expect(page.getByRole('heading', { name: 'Welcome to Arc', exact: true })).toBeVisible();
      expect(await page.evaluate(() => (window as Window & { firstHeadings?: string[] }).firstHeadings?.[0])).toBe('Welcome to Arc');
      const name = 'Owner <b>& "Name"';
      expect((await page.request.post('/api/auth/setup', { data: { name, password: 'Arc-browser-test-123', local_bypass: false } })).ok()).toBe(true);
      await page.reload();
      await expect(page.getByRole('heading', { name: 'Welcome to Arc', exact: true })).toBeVisible();
      await page.setViewportSize({ width: 390, height: 900 });
      await expect(page.getByRole('button', { name: `Profile: ${name}`, exact: true })).toBeVisible();
      await expect(page.locator('b')).toHaveCount(0);
      expect(localeRequests).toEqual([]);
      expect(errors).toEqual([]);
    });
  });
}
