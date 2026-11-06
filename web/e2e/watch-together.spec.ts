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


// Watch Together on the web (docs/WATCH-TOGETHER-API.md): two separate devices in one room. The
// host starts it from the title's ⋯ menu; the other joins with the code from the profile menu.
// Play, pause and seek on one move the other; leaving shows on the host's people count.
async function keepPlaying(page: Page) {
  const cdp = await page.context().newCDPSession(page);
  await cdp.send('Emulation.setFocusEmulationEnabled', { enabled: true });
}
const videoState = (page: Page) => page.locator('video').evaluate((v: HTMLVideoElement) => ({ paused: v.paused, at: v.currentTime }));

test('watch together: a second device joins with the invite link, waits in the lobby and follows play, pause and seek', async ({ page, browser }, info) => {
  const { item } = await seed(page);
  await page.goto('/movie/' + item.id);
  await keepPlaying(page);
  await page.getByRole('button', { name: 'More', exact: true }).first().click();
  await page.getByRole('menuitem', { name: 'Watch Together', exact: true }).click();
  // The host's player opens in the room, paused, on the lobby: the invite code and link, and Start.
  const lobby = page.getByRole('dialog', { name: 'Watch Together' });
  await expect(lobby).toBeVisible({ timeout: 20_000 });
  await expect(lobby.getByText('1 person here', { exact: true })).toBeVisible();
  await expect(lobby.getByRole('button', { name: 'Copy invite link', exact: true })).toBeVisible();
  const codeButton = lobby.getByRole('button', { name: /^Copy code / });
  const code = (await codeButton.getAttribute('aria-label'))!.replace('Copy code ', '');
  expect(code).toMatch(/^[A-Z0-9]{4}-[A-Z0-9]{4}$/);
  await expect(page.getByRole('button', { name: /^Watch Together: 1 person$/ })).toBeVisible();

  // A second device: its own sign-in (another session), same household profile, opens the invite link.
  const other = await browser.newContext();
  await other.route('**/*', route => { const u = new URL(route.request().url()); return u.hostname === '127.0.0.1' || u.protocol === 'data:' ? route.continue() : route.abort(); });
  const login = await other.request.post('http://127.0.0.1:18199/api/auth/login', { data: { password } });
  expect(login.ok()).toBe(true);
  const status = await (await other.request.get('http://127.0.0.1:18199/api/auth/status')).json();
  const chosen = await other.request.post('http://127.0.0.1:18199/api/auth/profile', { data: { id: status.profiles[0].id } });
  expect(chosen.ok()).toBe(true);
  const guest = await other.newPage();
  await guest.goto(`http://127.0.0.1:18199/?together=${code}`);
  await keepPlaying(guest);
  await expect(guest.getByRole('textbox', { name: 'Code', exact: true })).toHaveValue(code);
  await expect(guest).toHaveURL('http://127.0.0.1:18199/');
  await guest.screenshot({ path: info.outputPath('join-dialog.png') });
  await guest.getByRole('button', { name: 'Join', exact: true }).click();

  // The guest waits for the host; the host's lobby shows them arrive.
  const guestLobby = guest.getByRole('dialog', { name: 'Watch Together' });
  await expect(guestLobby.getByText(/^Waiting for .+ to start…$/)).toBeVisible({ timeout: 20_000 });
  await expect(lobby.getByText('2 people here', { exact: true })).toBeVisible({ timeout: 10_000 });
  await page.screenshot({ path: info.outputPath('lobby-host.png') });
  await guest.screenshot({ path: info.outputPath('lobby-guest.png') });
  await expect.poll(async () => (await videoState(guest)).paused, { timeout: 15_000 }).toBe(true);

  // The host starts: both lobbies close and both players play.
  await lobby.getByRole('button', { name: 'Start', exact: true }).click();
  await expect(lobby).toBeHidden({ timeout: 10_000 });
  await expect(guestLobby).toBeHidden({ timeout: 10_000 });
  await expect.poll(async () => { const s = await videoState(guest); return !s.paused && s.at > 1; }, { timeout: 20_000 }).toBe(true);
  await expect.poll(async () => { const s = await videoState(page); return !s.paused && s.at > 1; }, { timeout: 20_000 }).toBe(true);

  // The guest pauses: the host pauses too, and is told who did it.
  await guest.locator('video').hover();
  await guest.getByRole('button', { name: 'Pause (Space)', exact: true }).click();
  await expect.poll(async () => (await videoState(page)).paused, { timeout: 10_000 }).toBe(true);
  await expect(page.getByText(/^Paused by /)).toBeVisible({ timeout: 5_000 });
  await page.screenshot({ path: info.outputPath('paused-by.png') });

  // Two quick seeks on the host (5 s each, adding up) move the guest there, still paused.
  await page.locator('video').hover();
  const before = (await videoState(page)).at;
  await page.keyboard.press('ArrowRight'); // forward 5 s
  await page.keyboard.press('ArrowRight');
  await expect.poll(async () => {
    const [h, g] = [await videoState(page), await videoState(guest)];
    return g.paused && Math.abs(h.at - g.at) < 1 && g.at > before + 9;
  }, { timeout: 20_000 }).toBe(true);

  // The guest closes the player: that leaves the room, and the host sees one person again.
  await guest.getByRole('button', { name: 'Close player', exact: true }).click();
  await expect(page.getByRole('button', { name: /^Watch Together: 1 person$/ })).toBeVisible({ timeout: 10_000 });
  await other.close();
});
