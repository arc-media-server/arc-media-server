import { test, expect, type Page } from '@playwright/test';
import type { Item, Library } from '../src/lib/api';
type Fixtures = { movies: string; shows: string; empty: string; playback: string };
const password = 'Arc-browser-test-123';
let media: Fixtures;

test.beforeEach(async ({ page }) => {
  const r = await fetch('http://127.0.0.1:18200/reset', { method: 'POST' });
  if (!r.ok) throw new Error(await r.text());
  media = await r.json();
  await page.context().route('**/*', route => { const u = new URL(route.request().url()); return u.hostname === '127.0.0.1' || u.protocol === 'data:' ? route.continue() : route.abort(); });
  await setup(page);
});
async function seed(page: Page, shows = false) {
  const r = await page.request.post('/api/libraries', { data: { name: shows ? 'Audit TV' : 'Audit Movies', kind: shows ? 'shows' : 'movies', paths: [shows ? media.shows : media.movies] } });
  expect(r.ok()).toBe(true); const lib: Library = await r.json();
  await expect.poll(async () => (await (await page.request.get('/api/libraries/' + lib.id + '/scan')).json()).running, { timeout: 30000 }).toBe(false);
  const items: Item[] = (await (await page.request.get('/api/items?library_id=' + lib.id)).json()).items;
  return { lib, item: items.find((i: Item) => i.title === (shows ? 'Arc Show' : 'Arc Movie'))!, items };
}
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

test('Home cards share library actions and scope bulk selection to loaded row titles', async ({ page, browser }, info) => {
  const { item } = await seed(page);
  await page.goto('/');
  const card = page.locator('[data-item="' + item.id + '"]').filter({ visible: true }).first();
  await card.hover();
  await expect(card.getByRole('button', { name: 'Play ' + item.title, exact: true })).toBeVisible();
  await card.getByRole('button', { name: 'Edit ' + item.title, exact: true }).click();
  await expect(page.getByRole('dialog', { name: 'Edit', exact: true })).toBeVisible();
  await page.getByRole('dialog').getByRole('button', { name: 'Cancel', exact: true }).click();
  await card.getByRole('button', { name: 'More for ' + item.title, exact: true }).click();
  await expect(page.getByRole('menuitem', { name: /mark as watched/i })).toBeVisible();
  await page.keyboard.press('Escape');
  await card.getByRole('button', { name: 'Select ' + item.title, exact: true }).click();
  await expect(page.getByText(/^Loaded titles in /)).toBeVisible();
  await expect(page.getByText('1 selected', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Watched', exact: true }).click();
  await expect.poll(async () => (await (await page.request.get('/api/items/' + item.id)).json()).user_data.played).toBe(true);
  await page.getByRole('button', { name: 'Stop choosing', exact: true }).click();
  await expect(page.getByText(/^Loaded titles in /)).toHaveCount(0);
  await page.setViewportSize({ width: 390, height: 844 });
  const touch = await browser.newContext({ storageState: await page.context().storageState(), viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true });
  const phone = await touch.newPage();
  await phone.goto('http://127.0.0.1:18199/');
  const phoneCard = phone.locator('[data-item="' + item.id + '"]').filter({ visible: true }).first();
  await expect(phoneCard.getByRole('button', { name: 'More for ' + item.title, exact: true })).toBeVisible();
  await phone.screenshot({ path: info.outputPath('home-actions-phone.png'), fullPage: true });
  const viewer = await (await phone.request.post('/api/profiles', { data: { name: 'Viewer' } })).json();
  await phone.request.post('/api/auth/profile', { data: { id: viewer.id } });
  await phone.reload();
  await expect(phoneCard.getByRole('button', { name: 'Edit ' + item.title, exact: true })).toHaveCount(0);
  await expect(phoneCard.getByRole('button', { name: 'Select ' + item.title, exact: true })).toHaveCount(0);
  await expect(phoneCard.getByRole('button', { name: 'More for ' + item.title, exact: true })).toBeVisible();
  await touch.close();
});
