import { test, expect } from '@playwright/test';

// Opening Arc afresh starts at "Who's watching?"; "Sign in automatically" skips it in this browser.
// This journey starts without the "already open" mark the other journeys share (playwright.config.ts).
test.use({ storageState: { cookies: [], origins: [] } });

test('fresh start opens "Who\'s watching?"; Sign in automatically skips it; a reload never does', async ({ page }, info) => {
  const reset = await fetch('http://127.0.0.1:18200/reset', { method: 'POST' });
  expect(reset.ok).toBeTruthy();
  expect((await page.request.post('/api/auth/setup', { data: { name: 'Launch owner', password: 'Fresh-start-fixture-123', local_bypass: false, tmdb_api_key: 'fixture-not-a-secret' } })).ok()).toBeTruthy();
  expect((await page.request.patch('/api/settings', { data: { scan_automatic: false, scan_on_start: false, scan_interval_minutes: 0 } })).ok()).toBeTruthy();
  await page.emulateMedia({ colorScheme: 'dark' });
  const picker = page.getByRole('heading', { name: "Who's watching?", level: 1 });
  const home = page.getByRole('heading', { name: 'Home', exact: true, level: 1 });
  // Leaving a page marks Arc as still open, so the mark is cleared as the next page starts.
  let starts = 0;
  const freshStart = async () => {
    const n = ++starts;
    await page.addInitScript((n) => {
      if (Number(sessionStorage.getItem('fresh-start') ?? 0) < n) {
        sessionStorage.setItem('fresh-start', String(n));
        localStorage.setItem('arc-alive', '1');
      }
    }, n);
    await page.reload();
  };

  // Setup left a profile in use, but a fresh start still asks.
  await page.goto('/');
  await expect(picker).toBeVisible();
  await page.screenshot({ path: info.outputPath('picker.png') });
  await page.getByRole('button', { name: 'Launch owner' }).click();
  await expect(home).toBeVisible();

  // A reload (or another tab while Arc is open) isn't a fresh start.
  await page.reload();
  await expect(home).toBeVisible();

  // Sign in automatically: the next fresh start opens straight into the profile.
  await page.goto('/settings');
  const toggle = page.getByRole('switch', { name: 'Sign in automatically', exact: true });
  await toggle.click();
  await expect(toggle).toBeChecked();
  await page.getByText('Sign in automatically', { exact: true }).scrollIntoViewIfNeeded();
  await page.screenshot({ path: info.outputPath('setting.png') });
  await freshStart();
  await expect(page.getByRole('switch', { name: 'Sign in automatically', exact: true })).toBeChecked();
  await expect(picker).toHaveCount(0);

  // Turned off again: back to "Who's watching?".
  await page.getByRole('switch', { name: 'Sign in automatically', exact: true }).click();
  await freshStart();
  await expect(picker).toBeVisible();
});
