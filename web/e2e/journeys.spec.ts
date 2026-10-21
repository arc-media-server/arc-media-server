import { test as base, expect, type Page } from '@playwright/test';

type Fixtures = { movies: string; shows: string; empty: string; playback: string };
const password = 'Arc-browser-test-123';

const test = base.extend<{ media: Fixtures }>({
  media: async ({ page }, use) => {
    const reset = await fetch('http://127.0.0.1:18200/reset', { method: 'POST' });
    if (!reset.ok) throw new Error(await reset.text());
    const errors: string[] = [];
    page.on('pageerror', e => errors.push(e.message));
    // Tests need no internet access; third-party scripts/images must not hide dependencies.
    await page.context().route('**/*', route => {
      const u = new URL(route.request().url());
      return u.hostname === '127.0.0.1' || u.protocol === 'data:' ? route.continue() : route.abort();
    });
    await use(await reset.json());
    expect(errors, 'uncaught browser errors').toEqual([]);
  },
});

test('restoring a backup immediately restores running settings and access controls', async ({ page, media }) => {
  void media;
  await setup(page);
  const before = { region: 'GB', local_bypass: false, max_transcodes: 1 };
  expect((await page.request.patch('/api/settings', { data: before })).ok()).toBeTruthy();
  const backup = await page.request.post('/api/backups');
  expect(backup.ok()).toBeTruthy();
  const { name } = await backup.json();
  expect((await page.request.patch('/api/settings', {
    data: { region: 'US', local_bypass: true, max_transcodes: 3 },
  })).ok()).toBeTruthy();
  const changed = await (await page.request.get('/api/settings')).json();
  expect(changed).toMatchObject({ region: 'US', local_bypass: true, max_transcodes: 3 });
  const restored = await page.request.post(`/api/backups/${name}/restore`);
  expect(restored.ok(), await restored.text()).toBeTruthy();
  const settings = await (await page.request.get('/api/settings')).json();
  expect(settings).toMatchObject(before);
  await page.reload();
  expect((await (await page.request.get('/api/auth/status')).json()).bypass).toBe(false);
});

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

for (const method of ['remux', 'transcode'] as const) {
  test(`real ${method} stream plays, seeks and resumes`, async ({ page, media }) => {
    await setup(page);
    expect((await page.request.patch('/api/settings', { data: { hw_accel: false, max_transcodes: 1 } })).ok()).toBeTruthy();
    await addLibrary(page, 'Playback fixtures', media.playback);
    await page.locator('aside').getByRole('link', { name: 'Playback fixtures', exact: true }).click();
    await page.getByRole('link', { name: method === 'remux' ? /Remux Movie/ : /Transcode Movie/ }).first().click();
    const cdp = await page.context().newCDPSession(page);
    await cdp.send('Emulation.setFocusEmulationEnabled', { enabled: true });
    const plan = page.waitForResponse(r => r.url().endsWith('/playback') && r.ok());
    await page.getByRole('button', { name: 'Play', exact: true }).click();
    expect((await (await plan).json()).method).toBe(method);
    const video = page.locator('video');
    await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(1);
    for (let i = 0; i < 7; i++) await page.keyboard.press('l');
    // Converted streams have a relative video clock; the player's position includes offset_ms.
    await expect.poll(async () => Number(await page.getByRole('slider', { name: 'Position' }).getAttribute('aria-valuenow'))).toBeGreaterThan(65);
    await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.readyState)).toBeGreaterThanOrEqual(2);
    const saved = page.waitForResponse(r => r.url().endsWith('/progress') && r.request().method() === 'PUT'
      && r.request().postDataJSON()?.state === 'stopped' && r.ok());
    await page.getByRole('button', { name: 'Close player', exact: true }).click();
    expect((await (await saved).json()).position_ms).toBeGreaterThan(65_000);
    await page.reload();
    await page.getByRole('button', { name: /^Resume/ }).click();
    await expect.poll(async () => Number(await page.getByRole('slider', { name: 'Position' }).getAttribute('aria-valuenow'))).toBeGreaterThan(65);
    await expect.poll(() => page.locator('video').evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(1);
    await page.getByRole('button', { name: 'Close player', exact: true }).click();
  });
}

async function preview(page: Page, name: string, path: string, shows = false) {
  await page.goto('/libraries');
  await page.getByRole('button', { name: 'Add Library', exact: true }).click();
  const dialog = page.getByRole('dialog');
  if (shows) await dialog.getByRole('button', { name: 'TV Shows', exact: true }).click();
  await dialog.getByRole('textbox').first().fill(name);
  const folder = dialog.getByPlaceholder('Choose a drive, or type a folder path');
  await folder.fill(path);
  const listed = page.waitForResponse(r => r.url().includes('/api/folders?') && r.ok());
  await folder.press('Enter');
  await listed;
  await dialog.getByRole('button', { name: 'Use This Folder', exact: true }).click();
  const started = page.waitForResponse(r => r.url().endsWith('/api/library-previews') && r.request().method() === 'POST' && r.ok());
  await dialog.getByRole('button', { name: 'Preview', exact: true }).click();
  const { id } = await (await started).json();
  await expect(page.getByRole('dialog', { name: `Preview: ${name}`, exact: true })).toBeVisible();
  await expect.poll(async () => (await (await page.request.get(`/api/library-previews/${id}`)).json()).stage).toBe('done');
  return dialog;
}

async function addLibrary(page: Page, name: string, path: string, shows = false) {
  const dialog = await preview(page, name, path, shows);
  const created = page.waitForResponse(r => r.url().endsWith('/api/libraries') && r.request().method() === 'POST' && r.ok());
  await dialog.getByRole('button', { name: 'Add Library', exact: true }).click();
  const library = await (await created).json();
  await expect(dialog).not.toBeVisible();
  await expect.poll(async () => (await (await page.request.get(`/api/libraries/${library.id}/scan`)).json()).running,
    { timeout: 30_000 }).toBe(false);
  return library as { id: number };
}

async function profileMenu(page: Page, name: string) {
  await page.locator('aside').getByRole('button', { name: new RegExp(name) }).click();
}

test('setup validation, wrong password, sign-out and fresh sign-in', async ({ page, media }) => {
  void media;
  await page.goto('/');
  await page.getByLabel('Your name', { exact: true }).fill('Admin');
  await page.getByLabel('Password', { exact: true }).fill('short');
  await page.getByLabel('Password again', { exact: true }).fill('different');
  await expect(page.getByRole('button', { name: 'Continue', exact: true })).toBeDisabled();
  await setup(page);
  await profileMenu(page, 'Admin');
  await page.getByRole('menuitem', { name: 'Sign Out', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Sign in', exact: true })).toBeVisible();
  await page.getByLabel('Password', { exact: true }).fill('incorrect-password');
  await page.getByRole('button', { name: 'Sign In', exact: true }).click();
  await expect(page.getByText("that password isn't right", { exact: true })).toBeVisible();
  await page.getByLabel('Password', { exact: true }).fill(password);
  await page.getByRole('button', { name: 'Sign In', exact: true }).click();
  await expect(page.getByRole('heading', { name: "Who's watching?" })).toBeVisible();
  await page.getByRole('button', { name: 'Admin', exact: true }).click();
  await expect(page.locator('aside')).toBeVisible();
});

test('movie preview, import and manually fixing an unmatched title persist', async ({ page, media }) => {
  await setup(page);
  const lib = await addLibrary(page, 'Test Movies', media.movies);
  await page.locator('aside').getByRole('link', { name: 'Test Movies', exact: true }).click();
  await page.getByRole('link', { name: /Unmatched Mystery/ }).first().click();
  await expect(page.getByRole('heading', { name: 'Unmatched Mystery', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Identify it…', exact: true }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Title', { exact: true }).fill('Resolved Movie');
  await dialog.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(dialog.getByText('Resolved Movie', { exact: false }).last()).toBeVisible();
  await dialog.getByRole('button', { name: 'Use This', exact: true }).click();
  await expect(dialog).not.toBeVisible();
  await expect(page.getByRole('heading', { name: 'Resolved Movie', exact: true })).toBeVisible();
  await page.reload();
  await expect(page.getByRole('heading', { name: 'Resolved Movie', exact: true })).toBeVisible();
  const items = await (await page.request.get(`/api/items?library_id=${lib.id}`)).json();
  expect(items.items.map((i: { title: string }) => i.title)).toEqual(expect.arrayContaining(['Arc Movie', 'Resolved Movie']));
});

test('empty previews and failed preview requests leave the app usable', async ({ page, media }) => {
  await setup(page);
  const dialog = await preview(page, 'Empty Library', media.empty);
  await expect(dialog.getByRole('button', { name: 'Add Library', exact: true })).toBeVisible();
  await expect(dialog.getByText('0', { exact: true }).first()).toBeVisible();
  await expect(page.locator('body')).not.toBeEmpty();
  await dialog.getByRole('button', { name: 'Back', exact: true }).click();
  await page.route('**/api/library-previews', route => route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ error: 'Fixture preview unavailable' }) }));
  await dialog.getByRole('button', { name: 'Preview', exact: true }).click();
  await expect(dialog.getByText('Fixture preview unavailable', { exact: true })).toBeVisible();
  await expect(dialog.getByRole('button', { name: 'Preview', exact: true })).toBeEnabled();
});

test('alternate episode order persists and unplaced files remain visible', async ({ page, media }) => {
  await setup(page);
  await addLibrary(page, 'Test Shows', media.shows, true);
  await page.locator('aside').getByRole('link', { name: 'Test Shows', exact: true }).click();
  await page.getByRole('link', { name: /Arc Show/ }).first().click();
  const first = page.getByText('First Chapter', { exact: true });
  const second = page.getByText('Second Chapter', { exact: true });
  await expect(first).toBeVisible();
  await expect(second).toBeVisible();
  expect((await first.boundingBox())!.y).toBeLessThan((await second.boundingBox())!.y);
  await page.getByRole('button', { name: /Aired Order/ }).click();
  await page.getByRole('menuitemradio', { name: /Story Order/ }).click();
  await expect(page.getByRole('button', { name: /Story Order/ })).toBeVisible();
  await expect.poll(async () => (await second.boundingBox())!.y < (await first.boundingBox())!.y).toBe(true);
  await page.reload();
  await expect(page.getByRole('button', { name: /Story Order/ })).toBeVisible();
  await expect(second).toBeVisible();
  expect((await second.boundingBox())!.y).toBeLessThan((await first.boundingBox())!.y);
  await page.getByRole('button', { name: /Not in This Order/ }).click();
  await expect(page.getByText('Episode 99', { exact: true })).toBeVisible();
});

test('profile switching enforces library access and protects admin pages', async ({ page, media }) => {
  await setup(page);
  const movies = await addLibrary(page, 'Private Movies', media.movies);
  await addLibrary(page, 'Allowed Shows', media.shows, true);
  const hidden = (await (await page.request.get(`/api/items?library_id=${movies.id}`)).json()).items[0];
  await page.locator('aside').getByRole('link', { name: 'Profiles', exact: true }).click();
  await page.getByRole('button', { name: 'Add Profile', exact: true }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByLabel('Name', { exact: true }).fill('Viewer');
  await dialog.getByRole('button', { name: 'Add PIN', exact: true }).click();
  await dialog.getByPlaceholder('4 digits').fill('1234');
  await dialog.getByRole('switch').last().uncheck();
  await dialog.getByRole('checkbox', { name: 'Private Movies', exact: true }).uncheck();
  await dialog.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(dialog).not.toBeVisible();
  await profileMenu(page, 'Admin');
  await page.getByRole('menuitem', { name: 'Switch Profile', exact: true }).click();
  await page.getByRole('button', { name: 'Viewer', exact: true }).click();
  const rejected = page.waitForResponse(r => r.url().endsWith('/api/auth/profile') && r.status() === 401);
  await page.getByLabel('PIN', { exact: true }).fill('0000');
  await rejected;
  await expect(page.getByText('Try again', { exact: true })).toBeVisible();
  await page.getByLabel('PIN', { exact: true }).fill('1234');
  await expect(page.locator('aside').getByRole('link', { name: 'Allowed Shows', exact: true })).toBeVisible();
  await expect(page.locator('aside').getByRole('link', { name: 'Private Movies', exact: true })).toHaveCount(0);
  expect((await page.request.get('/api/profiles')).status()).toBe(403);
  await page.goto(`/movie/${hidden.id}`);
  await expect(page.getByText('item not found', { exact: true })).toBeVisible();
  await page.goto('/profiles');
  await expect(page.getByText('only the admin profile can do this', { exact: true })).toBeVisible();
});

test('real video plays, saves progress on close and resumes after reload', async ({ page, media }, testInfo) => {
  await setup(page);
  await addLibrary(page, 'Test Movies', media.movies);
  await page.locator('aside').getByRole('link', { name: 'Test Movies', exact: true }).click();
  await page.getByRole('link', { name: /Arc Movie/ }).first().click();
  const cdp = await page.context().newCDPSession(page);
  await cdp.send('Emulation.setFocusEmulationEnabled', { enabled: true });
  const plan = page.waitForResponse(r => r.url().endsWith('/playback') && r.ok());
  await page.getByRole('button', { name: 'Play', exact: true }).click();
  const decision = await (await plan).json();
  expect(decision.method).toBe('direct');
  const video = page.locator('video');
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(0);
  // Exercise the player's real seek handler, not a fabricated progress API request.
  // Arc deliberately keeps resume points only after the first minute.
  for (let i = 0; i < 7; i++) await page.keyboard.press('l');
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(65);
  const saved = page.waitForResponse(r => r.url().endsWith('/progress') && r.request().method() === 'PUT'
    && r.request().postDataJSON()?.state === 'stopped' && r.ok());
  await page.getByRole('button', { name: 'Close player', exact: true }).click();
  expect((await (await saved).json()).position_ms).toBeGreaterThan(65_000);
  await page.reload();
  const resume = page.getByRole('button', { name: /^Resume/ });
  await expect(resume).toBeVisible();
  const screenshot = testInfo.outputPath('saved-resume.png');
  await page.screenshot({ path: screenshot });
  await testInfo.attach('Saved resume point after reload', { path: screenshot, contentType: 'image/png' });
  await resume.click();
  await expect.poll(() => page.locator('video').evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(65);
  await page.getByRole('button', { name: 'Close player', exact: true }).click();
});
