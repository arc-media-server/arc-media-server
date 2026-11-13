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


// Watch Together on the web (docs/WATCH-TOGETHER-API.md), with two separate signed-in devices.
// The host picks who to invite from the title's ⋯ menu; the room appears on their Home in the
// Watch Together row, live, and both join from there into the lobby. Play, pause and seek on one
// move the other. A code and invite link still work alongside.
async function keepPlaying(page: Page) {
  const cdp = await page.context().newCDPSession(page);
  await cdp.send('Emulation.setFocusEmulationEnabled', { enabled: true });
}
const videoState = (page: Page) => page.locator('video').evaluate((v: HTMLVideoElement) => ({ paused: v.paused, at: v.currentTime }));

/** A second device signed in as `profile` (another session). */
async function device(browser: import('@playwright/test').Browser, profile: string) {
  const other = await browser.newContext();
  await other.route('**/*', route => { const u = new URL(route.request().url()); return u.hostname === '127.0.0.1' || u.protocol === 'data:' ? route.continue() : route.abort(); });
  const login = await other.request.post('http://127.0.0.1:18199/api/auth/login', { data: { password } });
  expect(login.ok()).toBe(true);
  const chosen = await other.request.post('http://127.0.0.1:18199/api/auth/profile', { data: { id: profile } });
  expect(chosen.ok()).toBe(true);
  const guest = await other.newPage();
  await keepPlaying(guest);
  return { other, guest };
}

test('watch together: the host invites someone, the room shows on both Homes, and they watch in step', async ({ page, browser }, info) => {
  const { item } = await seed(page);
  const sam = await (await page.request.post('/api/profiles', { data: { name: 'Sam', color: 'blue' } })).json();
  const { other, guest } = await device(browser, sam.id);
  // Sam's Home is open before the invitation, so it must arrive live.
  await guest.goto('http://127.0.0.1:18199/');
  await expect(guest.getByRole('heading', { name: /Recently Added/ }).first()).toBeVisible({ timeout: 20_000 });

  await page.goto('/movie/' + item.id);
  await keepPlaying(page);
  await page.getByRole('button', { name: 'More', exact: true }).first().click();
  await page.getByRole('menuitem', { name: 'Watch Together', exact: true }).click();
  const choose = page.getByRole('dialog', { name: 'Watch Together' });
  await expect(choose.getByText('Choose who to watch Arc Movie with', { exact: false })).toBeVisible();
  const done = choose.getByRole('button', { name: 'Done', exact: true });
  await expect(done).toBeDisabled();
  await choose.getByRole('checkbox', { name: /Sam/ }).click();
  await expect(choose.getByRole('checkbox', { name: /Sam/ })).toHaveAttribute('aria-checked', 'true');
  await page.screenshot({ path: info.outputPath('invite-dialog.png') });
  await done.click();
  await expect(choose).toBeHidden();
  await expect(page.getByText(/Your Watch Together room for Arc Movie is ready/)).toBeVisible();
  // Nobody is in the room yet: no player opened.
  await expect(page.locator('video')).toHaveCount(0);

  // Sam's Home shows the room without a reload.
  const row = guest.getByRole('region', { name: 'Watch Together', exact: true });
  const samCard = row.getByRole('article', { name: 'Arc Movie' });
  await expect(samCard).toBeVisible({ timeout: 15_000 });
  await expect(samCard.getByText('Hosted by Admin', { exact: true })).toBeVisible();
  await expect(samCard.getByText('Invited: Sam', { exact: true })).toBeVisible();
  await guest.screenshot({ path: info.outputPath('home-row-guest.png') });

  // The host's Home has it too.
  await page.goto('/');
  const hostCard = page.getByRole('region', { name: 'Watch Together', exact: true }).getByRole('article', { name: 'Arc Movie' });
  await expect(hostCard.getByText("You're hosting", { exact: true })).toBeVisible({ timeout: 15_000 });

  // Sam joins first and waits in the lobby for the host.
  await samCard.getByRole('button', { name: 'Join', exact: true }).click();
  const guestLobby = guest.getByRole('dialog', { name: 'Watch Together' });
  await expect(guestLobby.getByText('Waiting for Admin to start…', { exact: true })).toBeVisible({ timeout: 20_000 });
  // The host's card updates: Sam is there.
  await expect(hostCard.getByText('Sam is here', { exact: true })).toBeVisible({ timeout: 15_000 });

  // The host joins from Home: they're host, with Start, and see Sam.
  await hostCard.getByRole('button', { name: 'Join', exact: true }).click();
  const lobby = page.getByRole('dialog', { name: 'Watch Together' });
  await expect(lobby.getByText('2 people here', { exact: true })).toBeVisible({ timeout: 20_000 });
  await page.screenshot({ path: info.outputPath('lobby-host.png') });
  await expect.poll(async () => (await videoState(guest)).paused, { timeout: 15_000 }).toBe(true);
  await lobby.getByRole('button', { name: 'Start', exact: true }).click();
  await expect(lobby).toBeHidden({ timeout: 10_000 });
  await expect(guestLobby).toBeHidden({ timeout: 10_000 });
  await expect.poll(async () => { const s = await videoState(guest); return !s.paused && s.at > 1; }, { timeout: 20_000 }).toBe(true);
  await expect.poll(async () => { const s = await videoState(page); return !s.paused && s.at > 1; }, { timeout: 20_000 }).toBe(true);

  // Sam pauses: the host pauses too, and is told who did it.
  await guest.locator('video').hover();
  await guest.getByRole('button', { name: 'Pause (Space)', exact: true }).click();
  await expect.poll(async () => (await videoState(page)).paused, { timeout: 10_000 }).toBe(true);
  await expect(page.getByText('Paused by Sam', { exact: true })).toBeVisible({ timeout: 5_000 });
  await page.screenshot({ path: info.outputPath('paused-by.png') });

  // Two quick seeks on the host (5 s each, adding up) move Sam there, still paused.
  await page.locator('video').hover();
  const before = (await videoState(page)).at;
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('ArrowRight');
  await expect.poll(async () => {
    const [h, g] = [await videoState(page), await videoState(guest)];
    return g.paused && Math.abs(h.at - g.at) < 1 && g.at > before + 9;
  }, { timeout: 20_000 }).toBe(true);

  // Sam closes the player: that leaves the room, and the host sees one person again.
  await guest.getByRole('button', { name: 'Close player', exact: true }).click();
  await expect(page.getByRole('button', { name: /^Watch Together: 1 person$/ })).toBeVisible({ timeout: 10_000 });

  // The host leaves and cancels the room from Home: it goes from Sam's Home too.
  await page.getByRole('button', { name: 'Close player', exact: true }).click();
  await expect(hostCard.getByRole('button', { name: 'Cancel', exact: true })).toBeVisible({ timeout: 15_000 });
  await hostCard.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(hostCard).toBeHidden({ timeout: 10_000 });
  await expect(samCard).toBeHidden({ timeout: 15_000 });
  await other.close();
});

test('watch together: a code and invite link still work, through the lobby', async ({ page, browser }, info) => {
  const { item, lib } = await seed(page);
  void lib;
  await page.goto('/movie/' + item.id);
  await keepPlaying(page);
  await page.getByRole('button', { name: 'More', exact: true }).first().click();
  await page.getByRole('menuitem', { name: 'Watch Together', exact: true }).click();
  await page.getByRole('dialog', { name: 'Watch Together' }).getByRole('button', { name: 'Use a code instead', exact: true }).click();
  const lobby = page.getByRole('dialog', { name: 'Watch Together' });
  await expect(lobby.getByText('1 person here', { exact: true })).toBeVisible({ timeout: 20_000 });
  await lobby.getByText('Share a code or link instead', { exact: true }).click();
  await expect(lobby.getByRole('button', { name: 'Copy invite link', exact: true })).toBeVisible();
  const code = (await lobby.getByRole('button', { name: /^Copy code / }).getAttribute('aria-label'))!.replace('Copy code ', '');
  expect(code).toMatch(/^[A-Z0-9]{4}-[A-Z0-9]{4}$/);

  // Another device, same household profile, opens the invite link.
  const status = await (await page.request.get('/api/auth/status')).json();
  const { other, guest } = await device(browser, status.profiles[0].id);
  await guest.goto(`http://127.0.0.1:18199/?together=${code}`);
  await expect(guest.getByRole('textbox', { name: 'Code', exact: true })).toHaveValue(code);
  await expect(guest).toHaveURL('http://127.0.0.1:18199/');
  await guest.screenshot({ path: info.outputPath('join-dialog.png') });
  await guest.getByRole('button', { name: 'Join', exact: true }).click();
  const guestLobby = guest.getByRole('dialog', { name: 'Watch Together' });
  await expect(guestLobby.getByText(/^Waiting for .+ to start…$/)).toBeVisible({ timeout: 20_000 });
  await expect(lobby.getByText('2 people here', { exact: true })).toBeVisible({ timeout: 10_000 });
  await lobby.getByRole('button', { name: 'Start', exact: true }).click();
  await expect(guestLobby).toBeHidden({ timeout: 10_000 });
  await expect.poll(async () => { const s = await videoState(guest); return !s.paused && s.at > 1; }, { timeout: 20_000 }).toBe(true);
  await other.close();
});
