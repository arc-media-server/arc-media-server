// Prepares the shared screenshot library (tools/screenshot-library.json, docs/SCREENSHOTS.md) in a
// throwaway copy of Arc and photographs the web app for the README (docs/screenshots), or leaves
// it running for another Arc app to photograph. The library is well-known films and shows (copies
// of one short generated clip named after them) that Arc matches on TMDB/TheTVDB, plus some
// profiles and watching. Your own Arc data, settings and media aren't used.
//
//   cd web && npm run build && npx playwright install chromium
//   node tools/screenshots.mjs               # README screenshots into docs/screenshots
//   node tools/screenshots.mjs --serve       # leave Arc running on 127.0.0.1:18299 until Ctrl+C
//   node tools/screenshots.mjs --approve <code>   # approve an app's sign-in code on that server
//
// Keys come from AMS_TMDB_API_KEY (needed) and AMS_TVDB_API_KEY (optional) in the environment
// and go only to the scratch server. Needs Go and FFmpeg. --port changes the port; --state <file>
// (with --serve) writes the address, sign-in and item ids as JSON for scripts. Add --keep to leave
// the temporary folder (media, data) afterwards, or --media <folder> to use a prepared library
// (Movies and TV inside, named as in the library file). --chrome <path> uses that
// Chrome/Chromium instead of Playwright's own. AMS_TMDB_URL / AMS_TVDB_URL are passed on, to try
// the script against stand-in providers.
import { chromium, request } from 'playwright';
import { spawn, spawnSync } from 'node:child_process';
import { copyFile, mkdir, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseArgs } from 'node:util';

const { values: args } = parseArgs({ options: {
  out: { type: 'string', default: 'docs/screenshots' },
  serve: { type: 'boolean', default: false },
  approve: { type: 'string' },
  port: { type: 'string', default: '18299' },
  state: { type: 'string' },
  keep: { type: 'boolean', default: false },
  media: { type: 'string' },
  chrome: { type: 'string' },
} });

const repo = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const library = JSON.parse(await readFile(join(repo, 'tools/screenshot-library.json'), 'utf8'));
const port = Number(args.port);
const base = `http://127.0.0.1:${port}`;
const sleep = ms => new Promise(r => setTimeout(r, ms));

async function apiClient() {
  const api = await request.newContext({ baseURL: base });
  const call = async (method, path, data) => {
    const r = await api.fetch(path, { method, data });
    if (!r.ok()) throw new Error(`${method} ${path}: ${r.status()} ${await r.text()}`);
    return r.status() === 204 ? null : r.json().catch(() => null);
  };
  return { api, call };
}

// --approve: sign in to an already running screenshot server and approve an app's link code.
if (args.approve) {
  const { api, call } = await apiClient();
  await call('POST', '/api/auth/login', { password: library.admin.password });
  const { profiles } = await call('GET', '/api/auth/status').then(s => s ?? {});
  const admin = (profiles ?? []).find(p => p.name === library.admin.name);
  if (admin) await call('POST', '/api/auth/profile', { id: admin.id, password: library.admin.password });
  await call('POST', '/api/auth/link/approve', { code: args.approve });
  await api.dispose();
  console.log(`Approved ${args.approve}`);
  process.exit(0);
}

const tmdb = process.env.AMS_TMDB_API_KEY ?? '';
const tvdb = process.env.AMS_TVDB_API_KEY ?? '';
if (!tmdb) {
  console.error('Set AMS_TMDB_API_KEY (and optionally AMS_TVDB_API_KEY) in the environment first.');
  process.exit(1);
}

function run(command, argv, options = {}) {
  const r = spawnSync(command, argv, { cwd: repo, stdio: ['ignore', 'inherit', 'inherit'], ...options });
  if (r.error || r.status !== 0) throw new Error(`${command} ${argv.join(' ')} failed${r.error ? `: ${r.error.message}` : ''}`);
}

const root = await mkdtemp(join(tmpdir(), 'arc-screenshots-'));
let server;
let api;
async function cleanUp() {
  await api?.dispose().catch(() => {});
  if (server && server.exitCode === null) {
    server.kill();
    await new Promise(r => server.once('exit', r));
  }
  if (args.keep) console.log(`Kept ${root}`);
  else await rm(root, { recursive: true, force: true });
}

try {
  const media = args.media ? resolve(args.media) : join(root, 'media');
  console.log('Making the library...');
  const clip = join(root, 'clip.mp4');
  // Ten seconds of 1080p so file details read like a normal film; tiny on disk.
  run('ffmpeg', ['-v', 'error', '-y', '-f', 'lavfi', '-i', 'color=c=0x101018:s=1920x1080:r=24:d=10',
    '-f', 'lavfi', '-i', 'anullsrc=r=48000:cl=stereo', '-t', '10', '-c:v', 'libx264', '-preset', 'ultrafast',
    '-tune', 'stillimage', '-c:a', 'aac', '-shortest', clip]);
  for (const m of args.media ? [] : library.movies) {
    await mkdir(join(media, 'Movies', m), { recursive: true });
    await copyFile(clip, join(media, 'Movies', m, `${m}.mp4`));
  }
  for (const { folder, episodes } of args.media ? [] : library.shows) {
    const name = folder.replace(/ \(\d{4}\)$/, '');
    await mkdir(join(media, 'TV', folder, 'Season 01'), { recursive: true });
    for (let e = 1; e <= episodes; e++) {
      await copyFile(clip, join(media, 'TV', folder, 'Season 01', `${name} - S01E${String(e).padStart(2, '0')}.mp4`));
    }
  }

  console.log('Starting a throwaway Arc...');
  const binary = join(root, process.platform === 'win32' ? 'ams.exe' : 'ams');
  run('go', ['build', '-o', binary, './cmd/ams']);
  const env = Object.fromEntries(Object.entries(process.env).filter(([k]) => !k.toUpperCase().startsWith('AMS_') || /^AMS_(TMDB|TVDB)_URL$/.test(k)));
  server = spawn(binary, ['-data', join(root, 'data'), '-config', 'none'], {
    cwd: root, env: { ...env, AMS_LISTEN: `127.0.0.1:${port}` }, stdio: 'ignore', windowsHide: true,
  });
  for (let i = 0; ; i++) {
    try { if ((await fetch(`${base}/api/health`)).ok) break; } catch {}
    if (i > 200 || server.exitCode !== null) throw new Error(`Arc did not start (is port ${port} free?)`);
    await sleep(100);
  }

  const client = await apiClient();
  api = client.api;
  const { call } = client;
  await call('POST', '/api/auth/setup', { name: library.admin.name, password: library.admin.password, local_bypass: true,
    tmdb_api_key: tmdb, tvdb_api_key: tvdb, region: 'GB' });
  const movieLib = await call('POST', '/api/libraries', { name: 'Movies', kind: 'movies', paths: [join(media, 'Movies')] });
  const tvLib = await call('POST', '/api/libraries', { name: 'TV Shows', kind: 'shows', paths: [join(media, 'TV')],
    source_ordering: tvdb ? 'tvdb_aired' : 'tmdb_aired' });

  console.log('Waiting for Arc to match everything...');
  for (const lib of [movieLib, tvLib]) {
    for (let i = 0; ; i++) {
      await sleep(2000);
      const scan = await call('GET', `/api/libraries/${lib.id}/scan`);
      if (!scan.running) break;
      if (i > 300) throw new Error(`${lib.name} is still scanning after 10 minutes`);
    }
  }
  await sleep(5000); // artwork caching finishes just after the scan

  const find = async (lib, title, quiet = false) => {
    const r = await call('GET', `/api/items?library_id=${lib.id}&q=${encodeURIComponent(title)}&limit=5`);
    const items = r.items ?? r;
    const want = title.toLowerCase();
    const item = items.find(i => i.title.toLowerCase() === want) ?? items.find(i => i.title.toLowerCase().startsWith(want));
    if (!item && !quiet) console.warn(`  not found: ${title}`);
    return item;
  };
  // Continue Watching is ordered by when things were last played, to the second, so the library
  // lists them most recent first and they're written oldest first, a second apart.
  for (const { title, part } of [...library.watching].reverse()) {
    const it = await find(movieLib, title);
    const duration = (it?.runtime_min || 100) * 60_000;
    if (it) await call('PUT', `/api/items/${it.id}/progress`, { position_ms: Math.round(duration * part), duration_ms: duration, state: 'stopped' });
    await sleep(1100);
  }
  // Episodes watched, then the next one half-way (Continue Watching and Up Next).
  for (const { title, watched } of [...library.showProgress].reverse()) {
    const show = await find(tvLib, title);
    if (!show) continue;
    const seasons = await call('GET', `/api/series/${show.id}/seasons`);
    const episodes = (seasons.seasons ?? seasons).flatMap(s => s.episodes).filter(e => e.available !== false);
    for (const e of episodes.slice(0, watched)) await call('PUT', `/api/items/${e.id}/played`, { played: true });
    const next = episodes[watched];
    if (next) await call('PUT', `/api/items/${next.id}/progress`, { position_ms: 12 * 60_000, duration_ms: 25 * 60_000, state: 'stopped' });
    await sleep(1100);
  }
  for (const title of library.watchlist) {
    const it = (await find(movieLib, title, true)) ?? (await find(tvLib, title));
    if (it) await call('PUT', `/api/items/${it.id}/watchlist`, { on: true });
  }
  const collections = [];
  for (const { title, overview, movies } of library.collections) {
    const ids = (await Promise.all(movies.map(m => find(movieLib, m)))).filter(Boolean).map(i => i.id);
    collections.push(await call('POST', '/api/collections', { library_id: movieLib.id, kind: 'manual', title, overview, item_ids: ids }));
  }
  for (const { name, color, maxAge } of library.profiles) await call('POST', '/api/profiles', { name, color, ...(maxAge ? { max_age: maxAge } : {}) });

  const movie = await find(movieLib, library.featured.movie);
  const show = await find(tvLib, library.featured.show);

  if (args.serve) {
    const state = {
      address: base,
      admin: library.admin.name,
      password: library.admin.password,
      libraries: { movies: movieLib.id, shows: tvLib.id },
      featured: { movie: movie?.id ?? null, show: show?.id ?? null },
      collections: collections.map(c => c?.id).filter(Boolean),
    };
    if (args.state) await writeFile(resolve(args.state), JSON.stringify(state, null, 2) + '\n');
    console.log(`
Arc is ready at ${base} (the web app is signed in on this machine without a password).
  Admin profile: ${state.admin}, household password: ${state.password}
  Apps: sign in with the password (POST /api/v1/auth/login), or show a link code and run
        node tools/screenshots.mjs --approve <code>${port === 18299 ? '' : ` --port ${port}`}
  Movies library ${state.libraries.movies}, TV Shows library ${state.libraries.shows}
  Featured film ${library.featured.movie}: ${state.featured.movie}; show ${library.featured.show}: ${state.featured.show}
Press Ctrl+C to stop; the scratch copy is deleted${args.keep ? ' (kept with --keep)' : ''}.`);
    await new Promise(r => {
      process.once('SIGINT', r);
      process.once('SIGTERM', r);
      server.once('exit', r);
    });
    console.log('Stopping...');
  } else {
    console.log('Taking screenshots...');
    const out = resolve(repo, args.out);
    await mkdir(out, { recursive: true });
    const browser = await chromium.launch(args.chrome ? { executablePath: args.chrome } : {});
    const context = await browser.newContext({ baseURL: base, viewport: { width: 1600, height: 1000 }, colorScheme: 'dark',
      storageState: await api.storageState() });
    const page = await context.newPage();
    const shoot = async (path, file) => {
      await page.goto(path);
      await page.waitForLoadState('networkidle');
      // Off-screen images load lazily and may never finish, so wait a few seconds at most.
      await page.evaluate(() => Promise.race([new Promise(r => setTimeout(r, 5000)),
        Promise.all([...document.images].map(i => i.complete ? null : new Promise(r => { i.onload = i.onerror = r; })))]));
      await sleep(1500); // let entrance animations settle
      await page.screenshot({ path: join(out, file) });
      console.log(`  ${file}`);
    };
    await shoot('/', 'home.png');
    if (movie) await shoot(`/movie/${movie.id}`, 'movie.png');
    if (show) await shoot(`/series/${show.id}`, 'show.png');
    await shoot(`/library/${movieLib.id}`, 'library.png');
    await context.request.fetch('/api/auth/profile', { method: 'DELETE' });
    await shoot('/', 'profiles.png');
    await browser.close();
    console.log(`Done: ${out}`);
  }
} finally {
  await cleanUp();
}
