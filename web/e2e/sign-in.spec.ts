import { expect, test } from '@playwright/test';

test('sign-in shows both methods and opens the appropriate profile', async ({ page }, info) => {
  const serverPassword = 'Arc-household-fixture-123';
  const friendPassword = 'Arc-friend-fixture-123';
  expect((await fetch('http://127.0.0.1:18200/reset', { method: 'POST' })).ok).toBeTruthy();
  expect((await page.request.post('/api/auth/setup', {
    data: { name: 'Host', password: serverPassword, local_bypass: false },
  })).ok()).toBeTruthy();
  const invite = await page.request.post('/api/profiles', {
    data: { name: 'Guest', color: 'blue', friend: true },
  });
  expect(invite.ok()).toBeTruthy();
  const { invite_token: token } = await invite.json();
  expect((await page.request.post(`/api/invites/${encodeURIComponent(token)}`, {
    data: { password: friendPassword },
  })).ok()).toBeTruthy();
  expect((await page.request.post('/api/auth/logout')).ok()).toBeTruthy();

  await page.setViewportSize({ width: 1280, height: 800 });
  await page.emulateMedia({ colorScheme: 'dark' });
  await page.goto('/');
  const server = page.getByRole('button', { name: 'Server password', exact: true });
  const friend = page.getByRole('button', { name: 'Friend login', exact: true });
  const password = page.getByLabel('Password', { exact: true });
  const name = page.getByLabel('Name', { exact: true });
  const submit = page.getByRole('button', { name: 'Sign In', exact: true });

  await expect(server).toHaveAttribute('aria-pressed', 'true');
  await expect(friend).toBeVisible();
  await expect(name).toHaveCount(0);
  await page.locator('body').click({ position: { x: 10, y: 10 } });
  await page.screenshot({ path: info.outputPath('server-password.jpg'), animations: 'disabled' });

  await password.fill(serverPassword);
  await friend.click();
  await expect(friend).toHaveAttribute('aria-pressed', 'true');
  await expect(password).toHaveValue('');
  await expect(page.getByText('First time here? Open the invite link your host sent you.')).toBeVisible();
  await page.locator('body').click({ position: { x: 10, y: 10 } });
  await page.screenshot({ path: info.outputPath('friend-login.jpg'), animations: 'disabled' });
  await name.fill('Guest');
  await password.fill(friendPassword);
  await submit.click();
  await expect(page.getByRole('heading', { name: 'Home', exact: true })).toBeVisible();
  const status = await (await page.request.get('/api/auth/status')).json();
  expect(status.friend).toBe(true);
  expect(status.profile.name).toBe('Guest');

  expect((await page.request.post('/api/auth/logout')).ok()).toBeTruthy();
  await page.reload();
  await expect(server).toHaveAttribute('aria-pressed', 'true');
  await password.fill(serverPassword);
  await submit.click();
  await expect(page.getByRole('heading', { name: "Who's watching?" })).toBeVisible();
});
