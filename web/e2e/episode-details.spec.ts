import { test as base, expect, type Page } from '@playwright/test';

async function chooseDisplay(page: Page, label: RegExp) {
 const select=page.getByRole('combobox',{name:'Display order',exact:true});
 const value=await select.locator('option').filter({hasText:label}).first().getAttribute('value');
 if(!value)throw new Error('Display option missing');
 await select.selectOption(value);
}

/** Episode ordering is the server owner's, in Edit, Metadata & ordering. */
async function openOrdering(page: Page) {
 await page.getByRole('button',{name:'More',exact:true}).click();
 await page.getByRole('menuitem',{name:'Edit…',exact:true}).click();
 await page.getByRole('tab',{name:'Metadata & ordering',exact:true}).click();
}
import type { Item, SeasonsResponse, OrderingsResponse } from '../src/lib/api';

const test = base.extend<{ media: { specials: string } }>({
  media: async ({ page }, use) => {
    const reset = await fetch('http://127.0.0.1:18200/reset', { method: 'POST' });
    expect(reset.ok).toBe(true);
    const errors: string[] = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.context().route('**/*', route => {
      const url = new URL(route.request().url());
      return url.hostname === '127.0.0.1' || url.protocol === 'data:' ? route.continue() : route.abort();
    });
    expect((await page.request.post('/api/auth/setup', { data: { name: 'Owner', password: 'Episodes-fixture-123', local_bypass: false,
      tmdb_api_key: 'local-fixture-key', tvdb_api_key: 'local-fixture-key', region: 'GB' } })).ok()).toBeTruthy();
    expect((await page.request.patch('/api/settings', { data: { scan_automatic: false, scan_on_start: false, scan_interval_minutes: 0,
      preview_mode: 'off', detection_mode: 'off', analysis_enabled: false, metadata_refresh_enabled: false, auto_backups_enabled: false } })).ok()).toBeTruthy();
    await use(await reset.json());
    expect(errors, 'uncaught browser errors').toEqual([]);
  },
});

async function importShow(page: Page, path: string) {
  const r = await page.request.post('/api/libraries', { data: { name: 'Episode fixtures', kind: 'shows', source_ordering: 'tmdb_aired', paths: [path] } });
  expect(r.ok()).toBeTruthy(); const lib = await r.json();
  await expect.poll(async () => (await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json()).running).toBe(false);
  const series: Item = (await (await page.request.get(`/api/items?library_id=${lib.id}&kind=series`)).json()).items[0];
  expect(series.title).toBe('Fixture Show');
  const refresh = await page.request.post(`/api/series/${series.id}/refresh?deep=1`);
  expect(refresh.ok(), await refresh.text()).toBeTruthy();
  await expect.poll(async () => (await (await page.request.get(`/api/libraries/${lib.id}/scan`)).json()).running).toBe(false);
  const seasons: SeasonsResponse = await (await page.request.get(`/api/series/${series.id}/seasons`)).json();
  const episodes = seasons.seasons.flatMap(s => s.episodes);
  expect(episodes).toHaveLength(4);
  return { series, episodes };
}

test('metadata ordering: staged file order, Retry, scoped refresh and independent display on phone', async ({page,media},info)=>{
  const {series,episodes}=await importShow(page,media.specials);
  const initialOrders: OrderingsResponse = await (await page.request.get(`/api/series/${series.id}/orderings`)).json();
  const tvdb = initialOrders.orderings.find(o => o.source === 'tvdb_aired')!;
  expect(initialOrders.display_choice_id).toBe(0);
  const first=episodes.find(e=>e.title==='Opening')!;
  const before=await (await page.request.get(`/api/items/${first.id}`)).json();
  expect((await page.request.put(`/api/items/${first.id}/progress`,{data:{position_ms:70000,duration_ms:120000,state:'stopped'}})).ok()).toBeTruthy();
  await page.goto(`/series/${series.id}`);
  const open=()=>openOrdering(page);
  const dialog=page.getByRole('dialog',{name:'Edit',exact:true});
  await open();
  await expect(dialog.getByRole('combobox',{name:'Display order',exact:true})).toHaveValue(String(initialOrders.display_ordering_id));
  await expect(dialog.getByRole('combobox',{name:'Display order',exact:true}).locator('option[value="inherit"]')).toHaveCount(0);
  await expect(dialog.getByText('Details From',{exact:true})).toHaveCount(0);
  await expect(dialog.getByText(/come from the library’s metadata provider, TMDB/)).toBeVisible();
  await dialog.getByRole('combobox',{name:'File episode order',exact:true}).selectOption('tvdb_aired');
  await expect(dialog.getByRole('combobox',{name:'Display order',exact:true})).toHaveValue(String(tvdb.id));
  await dialog.getByRole('button',{name:'Cancel',exact:true}).click();
  expect((await (await page.request.get(`/api/items/${series.id}`)).json()).source_ordering??'').toBe('');
  let fail=true;
  await page.route(`**/api/items/${series.id}`,route=>route.request().method()==='PATCH'&&fail?route.fulfill({status:503,json:{error:'File order unavailable'}}):route.continue());
  await open();
  await dialog.getByRole('combobox',{name:'File episode order',exact:true}).selectOption('tvdb_aired');
  await dialog.getByRole('button',{name:'Save',exact:true}).click();
  await expect(dialog.getByRole('alert')).toContainText('File order unavailable');
  fail=false;await dialog.getByRole('button',{name:'Retry',exact:true}).click();
  await expect(dialog).not.toBeVisible();
  expect((await (await page.request.get(`/api/items/${series.id}`)).json()).source_ordering).toBe('tvdb_aired');
  const savedOrders: OrderingsResponse = await (await page.request.get(`/api/series/${series.id}/orderings`)).json();
  expect(savedOrders.display_choice_id).toBe(tvdb.id);
  const status=await (await page.request.get(`/api/libraries/${series.library_id}/scan`)).json();
  expect(status).toMatchObject({operation:'refresh',running:false,files_seen:0,files_probed:0});
  const after=await (await page.request.get(`/api/items/${first.id}`)).json();
  expect(after.user_data.position_ms).toBe(70000);
  expect(after.files.map((f:{id:number})=>f.id)).toEqual(before.files.map((f:{id:number})=>f.id));
  await page.setViewportSize({width:390,height:844});
  await open();
  await expect(dialog.getByRole('combobox',{name:'File episode order',exact:true})).toHaveValue('tvdb_aired');
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await dialog.getByRole('button',{name:'Save',exact:true}).click({trial:true});
  const boxes=await dialog.getByRole('combobox').evaluateAll(elements=>elements.map(element=>{
    const box=element.getBoundingClientRect();return {x:box.x,width:box.width};
  }));
  expect(boxes).toHaveLength(3);
  for(const box of boxes.slice(1)){
    expect(box.x).toBeCloseTo(boxes[0].x,0);expect(box.width).toBeCloseTo(boxes[0].width,0);
  }
  await page.screenshot({path:info.outputPath('show-metadata-ordering-phone.png')});
  await chooseDisplay(page,/TMDB.*Aired Order/);
  await dialog.getByRole('button',{name:'Save',exact:true}).click();
  await expect(dialog).not.toBeVisible();
  expect((await (await page.request.get(`/api/items/${series.id}`)).json()).source_ordering).toBe('tvdb_aired');
});

test('episode details: imported credits/counts, keyboard/phone, provider switching and preserved identity', async ({ page, media }, info) => {
  const { series, episodes } = await importShow(page, media.specials);
  const ep = episodes.find(e => e.title === 'Opening')!;
  const before = await (await page.request.get(`/api/items/${ep.id}`)).json();
  expect(before.credits.map((c: { name: string }) => c.name)).toEqual(['Fixture Guest', 'Fixture Director', 'Fixture Writer']);
  expect((await page.request.patch(`/api/items/${ep.id}`, { data: { title: 'My opening', overview: 'The opening episode follows the visitors as they arrive in town and discover an unexpected mystery. Their search brings old friends together and starts a journey with consequences for everyone.' } })).ok()).toBeTruthy();
  expect((await page.request.put(`/api/items/${ep.id}/progress`, { data: { position_ms: 70000, duration_ms: 120000, state: 'stopped' } })).ok()).toBeTruthy();
  await page.goto(`/series/${series.id}`);
  await expect(page.getByText('14 episodes', { exact: true })).toBeVisible();
  const details = page.getByRole('button', { name: 'Details for My opening', exact: true });
  const actions = details.locator('..');
  for (const width of [1440, 1280, 1100, 1024, 900, 640, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await details.scrollIntoViewIfNeeded();
    await details.hover();
    const boxes = await actions.getByRole('button').evaluateAll(buttons => buttons.map(button => {
      const r = button.getBoundingClientRect(); return { x: r.x, y: r.y, right: r.right, bottom: r.bottom };
    }));
    expect(boxes).toHaveLength(7);
    expect(Math.max(...boxes.map(b => b.y)) - Math.min(...boxes.map(b => b.y)), `single action row at ${width}px`).toBeLessThan(6);
    expect((await page.getByRole('heading', { name: 'My opening', exact: true }).locator('..').boundingBox())!.width, `readable episode text at ${width}px`).toBeGreaterThanOrEqual(180);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: info.outputPath(`episode-actions-${width}.png`) });
  }
  await page.setViewportSize({ width: 1440, height: 900 });
  await details.focus(); await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: 'My opening', exact: true });
  await expect(dialog.getByText('Fixture Director', { exact: true })).toBeVisible();
  await expect(dialog.getByText('Fixture Writer', { exact: true })).toBeVisible();
  await expect(dialog.getByText('Fixture Guest', { exact: true })).toBeVisible();
  await expect(page.locator('video')).toHaveCount(0);
  await page.screenshot({ path: info.outputPath('episode-details-desktop.png') });
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: info.outputPath('episode-details-phone.png') });
  await page.keyboard.press('Escape');
  expect((await page.request.patch(`/api/items/${series.id}`, { data: { source_ordering: 'tvdb_aired' } })).ok()).toBeTruthy();
  const changed = await (await page.request.get(`/api/items/${ep.id}`)).json();
  expect(changed.credits ?? []).toEqual([]);
  expect(changed.title).toBe('My opening'); expect(changed.user_data.position_ms).toBe(70000);
  expect(changed.files.map((f: { id: number }) => f.id)).toEqual(before.files.map((f: { id: number }) => f.id));
  await details.click();
  await expect(dialog).toContainText('No cast or crew supplied by the file episode order provider.');
  await expect(dialog.getByText('Fixture Guest', { exact: true })).toHaveCount(0);
  await page.keyboard.press('Escape');
  expect((await page.request.patch(`/api/items/${series.id}`, { data: { source_ordering: 'tmdb_aired' } })).ok()).toBeTruthy();
  const restored = await (await page.request.get(`/api/items/${ep.id}`)).json();
  expect(restored.credits).toHaveLength(3); expect(restored.title).toBe('My opening');
  await details.click();
  await expect(dialog.getByText('Fixture Guest', { exact: true })).toBeVisible();
});

test('episode details: unavailable request Retry and spoiler protection do not start playback', async ({ page, media }) => {
  const { series } = await importShow(page, media.specials);
  let fail = true;
  await page.route('**/api/items/*', route => route.request().method() === 'GET' && fail
    ? route.fulfill({ status: 503, json: { error: 'Episode details unavailable' } }) : route.continue());
  await page.goto(`/series/${series.id}`);
  await page.getByRole('button', { name: 'Details for Opening', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'Opening', exact: true });
  await expect(dialog.getByRole('alert')).toContainText('Episode details unavailable');
  fail = false; await dialog.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(dialog.getByText('Fixture Guest', { exact: true })).toBeVisible();
  await page.keyboard.press('Escape');
  expect((await page.request.patch('/api/me/prefs', { data: { hide_spoilers: true } })).ok()).toBeTruthy();
  await page.reload();
  await page.getByRole('button', { name: 'Details for Opening', exact: true }).click();
  await expect(dialog).toContainText('Description and guest appearances hidden until you watch it.');
  await expect(dialog.getByText('Fixture Guest', { exact: true })).toHaveCount(0);
  await expect(page.locator('video')).toHaveCount(0);
});

test('inline specials: imported hints, save Retry, traversal, unknown fallback and the owner’s choice for every profile', async ({ page, media }, info) => {
  const { series, episodes } = await importShow(page, media.specials);
  const first = episodes.find(e => e.title === 'Opening')!;
  const special = episodes.find(e => e.title === 'Interlude')!;
  const last = episodes.find(e => e.title === 'Return')!;
  await page.goto(`/series/${series.id}`);
  let fail = true;
  await page.route('**/api/series/*/specials', route => fail ? route.fulfill({ status: 503, json: { error: 'Specials save unavailable' } }) : route.continue());
  const menu = async () => { await openOrdering(page); };
  await menu(); await page.getByRole('combobox',{name:'Specials in seasons',exact:true}).selectOption('on'); await page.getByRole('button',{name:'Save',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('Specials save unavailable');
  expect((await (await page.request.get(`/api/series/${series.id}/orderings`)).json()).inline_specials).toBe(false);
  fail = false; await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Details for Interlude', exact: true })).toBeVisible();
  await expect(page.getByText('Special 1', { exact: true })).toBeVisible();
  const next = await (await page.request.get(`/api/items/${first.id}/next`)).json(); expect(next.id).toBe(special.id);
  expect((await (await page.request.get(`/api/items/${special.id}/next`)).json()).id).toBe(last.id);
  const ordered: SeasonsResponse = await (await page.request.get(`/api/series/${series.id}/seasons`)).json();
  expect(ordered.seasons[0].episodes.map(e => e.title)).toEqual(['Opening', 'Interlude', 'Return']);
  expect(ordered.seasons[1].episodes.map(e => e.title)).toEqual(['Unplaced Extra']);
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: info.outputPath('inline-specials-phone.png'), fullPage: true });
  const user = await (await page.request.post('/api/profiles', { data: { name: 'Other viewer' } })).json();
  expect((await page.request.post('/api/auth/profile', { data: { id: user.id } })).ok()).toBeTruthy();
  await page.reload();
  // Other profiles follow the owner's choice and can't change it.
  await expect(page.getByRole('button', { name: 'Details for Interlude', exact: true })).toBeVisible();
  expect((await page.request.patch(`/api/series/${series.id}/specials`, { data: { inline_specials: false } })).status()).toBe(403);
  expect((await (await page.request.get(`/api/series/${series.id}/orderings`)).json()).inline_specials).toBe(true);
});

test('season names and summaries are edited from Edit, Seasons, for the order being viewed', async ({ page, media }) => {
  const { series } = await importShow(page, media.specials);
  await page.goto(`/series/${series.id}`);
  // The season bar keeps playback and view controls only; editing lives in the Edit dialog.
  await expect(page.getByRole('button', { name: 'Show Missing', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Edit Season', exact: true })).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Episode ordering…', exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: 'More', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Edit…', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'Edit', exact: true });
  await dialog.getByRole('tab', { name: 'Seasons', exact: true }).click();
  await expect(dialog.getByText(/order you’re viewing/)).toBeVisible();
  await dialog.getByRole('button', { name: /Season 1 2 episodes/ }).click();
  await dialog.getByRole('textbox', { name: 'Name', exact: true }).fill('');
  await expect(dialog.getByRole('button', { name: 'Save', exact: true })).toBeDisabled();
  await dialog.getByRole('textbox', { name: 'Name', exact: true }).fill('The First Run');
  await dialog.getByRole('textbox', { name: 'Summary', exact: true }).fill('A summary written by hand.');
  let fail = true;
  await page.route(`**/api/series/${series.id}/seasons/1*`, route => route.request().method() === 'PATCH' && fail
    ? route.fulfill({ status: 503, json: { error: 'Season save unavailable' } }) : route.continue());
  await dialog.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(dialog.getByRole('alert')).toContainText('Season save unavailable');
  fail = false;
  await dialog.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(dialog).not.toBeVisible();
  await expect(page.getByRole('button', { name: 'The First Run', exact: true })).toBeVisible();
  await expect(page.getByText('A summary written by hand.', { exact: true })).toBeVisible();
  const saved: SeasonsResponse = await (await page.request.get(`/api/series/${series.id}/seasons`)).json();
  const season = saved.seasons.find(s => s.season_number === 1)!;
  expect(season.name).toBe('The First Run');
  expect([...season.locked_fields].sort()).toEqual(['name', 'overview']);
});

test('named display choices preserve inherited storage and a manual choice after a file-order edit wins', async ({ page, media }) => {
  const { series } = await importShow(page, media.specials);
  const url = `/api/series/${series.id}/orderings`;
  const initial: OrderingsResponse = await (await page.request.get(url)).json();
  const tmdb = initial.orderings.find(o => o.source === 'tmdb_aired')!;
  const tvdb = initial.orderings.find(o => o.source === 'tvdb_aired')!;
  let writes = 0;
  page.on('request', r => { if (r.method() === 'PUT' && r.url().endsWith('/display-ordering')) writes++; });
  await page.goto(`/series/${series.id}`);
  await openOrdering(page);
  const display = page.getByRole('combobox', { name: 'Display order', exact: true });
  await expect(display).toHaveValue(String(tmdb.id));
  await expect(display.locator('option').filter({ hasText: /file episode order|Library default/ })).toHaveCount(0);
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  expect(writes).toBe(0);
  expect((await (await page.request.get(url)).json()).display_choice_id).toBe(0);
  await openOrdering(page);
  await page.getByRole('combobox', { name: 'File episode order', exact: true }).selectOption('tvdb_aired');
  await expect(display).toHaveValue(String(tvdb.id));
  await display.selectOption(String(tmdb.id));
  await page.getByRole('button', { name: 'Compare orders…', exact: true }).click();
  const comparison = page.getByRole('dialog', { name: 'Compare Episode Orders', exact: true });
  await expect(comparison.getByRole('combobox').first()).toHaveValue(String(tmdb.id));
  await comparison.getByRole('button', { name: 'Close', exact: true }).last().click();
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByRole('dialog', { name: 'Edit', exact: true })).not.toBeVisible();
  expect((await (await page.request.get(`/api/items/${series.id}`)).json()).source_ordering).toBe('tvdb_aired');
  expect((await (await page.request.get(url)).json()).display_choice_id).toBe(tmdb.id);
  // Reopening and saving unrelated details must retain the explicitly different display.
  await openOrdering(page);
  await expect(display).toHaveValue(String(tmdb.id));
  await page.getByRole('tab', { name: 'Details', exact: true }).click();
  await page.getByRole('dialog', { name: 'Edit', exact: true }).getByRole('textbox').first().fill('Fixture Show edited');
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByRole('dialog', { name: 'Edit', exact: true })).not.toBeVisible();
  expect((await (await page.request.get(url)).json()).display_choice_id).toBe(tmdb.id);
});

test('a display save failure after a file-order refresh explains partial saves and Cancel preserves the actual saved state', async ({ page, media }) => {
  const { series } = await importShow(page, media.specials);
  const url = `/api/series/${series.id}/orderings`;
  const initial: OrderingsResponse = await (await page.request.get(url)).json();
  const tmdb = initial.orderings.find(o => o.source === 'tmdb_aired')!;
  expect((await page.request.put(`/api/series/${series.id}/display-ordering`, { data: { ordering_id: tmdb.id } })).ok()).toBe(true);
  await page.goto(`/series/${series.id}`);
  await openOrdering(page);
  await page.getByRole('combobox', { name: 'File episode order', exact: true }).selectOption('tvdb_aired');
  await page.route('**/display-ordering', r => r.fulfill({ status: 503, json: { error: 'Display write unavailable' } }));
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  const edit = page.getByRole('dialog', { name: 'Edit', exact: true });
  await expect(edit.getByRole('alert')).toContainText('The file episode order was saved');
  await expect(edit.getByRole('alert')).toContainText('Cancel closes this dialog without undoing changes already saved');
  await edit.getByRole('button', { name: 'Cancel', exact: true }).click();
  expect((await (await page.request.get(`/api/items/${series.id}`)).json()).source_ordering).toBe('tvdb_aired');
  expect((await (await page.request.get(url)).json()).display_choice_id).toBe(tmdb.id);
  await page.unroute('**/display-ordering');
  await openOrdering(page);
  await expect(page.getByRole('combobox', { name: 'File episode order', exact: true })).toHaveValue('tvdb_aired');
  await expect(page.getByRole('combobox', { name: 'Display order', exact: true })).toHaveValue(String(tmdb.id));
  await edit.getByRole('button', { name: 'Cancel', exact: true }).click();
  // Legacy inherited rows also display the new effective named order after a partial save.
  expect((await page.request.put(`/api/series/${series.id}/display-ordering`, { data: { ordering_id: null } })).ok()).toBe(true);
  await page.reload();
  await openOrdering(page);
  await page.getByRole('combobox', { name: 'File episode order', exact: true }).selectOption('tmdb_aired');
  await page.route('**/display-ordering', r => r.fulfill({ status: 503, json: { error: 'Display write unavailable' } }));
  await edit.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(edit.getByRole('alert')).toContainText('The file episode order was saved');
  await edit.getByRole('button', { name: 'Cancel', exact: true }).click();
  expect((await (await page.request.get(url)).json()).display_choice_id).toBe(0);
  await page.unroute('**/display-ordering');
  await openOrdering(page);
  await expect(page.getByRole('combobox', { name: 'File episode order', exact: true })).toHaveValue('tmdb_aired');
  await expect(page.getByRole('combobox', { name: 'Display order', exact: true })).toHaveValue(String(tmdb.id));
});

test('an unavailable named file display reports partial state and can recover to an available display on Retry', async ({ page, media }) => {
  const { series } = await importShow(page, media.specials);
  const url = `/api/series/${series.id}/orderings`;
  const initial: OrderingsResponse = await (await page.request.get(url)).json();
  const tvdb = initial.orderings.find(o => o.source === 'tvdb_aired')!;
  expect(initial.orderings.some(o => o.source === 'tvdb_dvd')).toBe(false);
  await page.goto(`/series/${series.id}`);
  await openOrdering(page);
  await page.getByRole('combobox', { name: 'File episode order', exact: true }).selectOption('tvdb_dvd');
  const display = page.getByRole('combobox', { name: 'Display order', exact: true });
  await expect(display.locator('option:checked')).toHaveText('TheTVDB DVD order');
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  const edit = page.getByRole('dialog', { name: 'Edit', exact: true });
  await expect(edit.getByRole('alert')).toContainText('TheTVDB DVD order is not available as a display order');
  expect((await (await page.request.get(`/api/items/${series.id}`)).json()).source_ordering).toBe('tvdb_dvd');
  await display.selectOption(String(tvdb.id));
  await edit.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(edit).not.toBeVisible();
  expect((await (await page.request.get(url)).json()).display_choice_id).toBe(tvdb.id);
});
