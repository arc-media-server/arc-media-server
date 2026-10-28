// Takes the README screenshots (docs/screenshots) from a throwaway copy of Arc. It makes a small
// library named after Creative Commons and public-domain films and shows (copies of one short
// generated clip), lets Arc match them on TMDB/TheTVDB with your keys, adds some profiles and
// watching, then photographs the main screens. Your own Arc data, settings and media aren't used.
//
//   cd web && npm run build && npx playwright install chromium
//   node tools/screenshots.mjs --tmdb <TMDB key> [--tvdb <TheTVDB key>]
//
// Keys can also come from AMS_TMDB_API_KEY / AMS_TVDB_API_KEY. Needs Go and FFmpeg. Port 18299
// must be free. Add --keep to leave the temporary folder (media, data) for a look afterwards, or
// --media <folder> to use a prepared library (Movies and TV inside, named as below) instead.
// --chrome <path> uses that Chrome/Chromium instead of Playwright's own.
// AMS_TMDB_URL / AMS_TVDB_URL are passed on, to try the script against stand-in providers.
import { chromium } from 'playwright';
import { spawn, spawnSync } from 'node:child_process';
import { copyFile, mkdir, mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseArgs } from 'node:util';

const { values: args } = parseArgs({ options: {
  tmdb: { type: 'string', default: process.env.AMS_TMDB_API_KEY ?? '' },
  tvdb: { type: 'string', default: process.env.AMS_TVDB_API_KEY ?? '' },
  out: { type: 'string', default: 'docs/screenshots' },
  keep: { type: 'boolean', default: false },
  media: { type: 'string' },
  chrome: { type: 'string' },
} });
if (!args.tmdb) {
  console.error('A TMDB API key is needed: node tools/screenshots.mjs --tmdb <key> [--tvdb <key>]');
  process.exit(1);
}

const repo = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const out = resolve(repo, args.out);
const port = 18299;
const base = `http://127.0.0.1:${port}`;

// Blender Studio's open films (Creative Commons) and films in the public domain, all with posters
// on TMDB (Charge and Coffee Run have none, so they'd show as grey tiles).
const movies = [
  'Big Buck Bunny (2008)', 'Sintel (2010)', 'Tears of Steel (2012)', 'Elephants Dream (2006)',
  'Cosmos Laundromat (2015)', 'Spring (2019)', 'Sprite Fright (2021)', 'The Gold Rush (1925)',
  'Steamboat Bill, Jr. (1928)', 'Agent 327 Operation Barbershop (2017)', 'Night of the Living Dead (1968)',
  'Nosferatu (1922)', 'Metropolis (1927)', 'The General (1926)', 'Sherlock Jr. (1924)', 'The Kid (1921)',
  'Charade (1963)', 'His Girl Friday (1940)', 'Plan 9 from Outer Space (1957)',
  'The Phantom of the Opera (1925)', 'A Trip to the Moon (1902)', 'The Cabinet of Dr. Caligari (1920)',
  'Detour (1945)', 'The Last Man on Earth (1964)',
];
// Shows with public-domain episodes: [folder, episodes in season 1].
const shows = [
  ['The Beverly Hillbillies (1962)', 8], ['Bonanza (1959)', 6], ['The Dick Van Dyke Show (1961)', 6],
  ['One Step Beyond (1959)', 6], ['The Lucy Show (1962)', 6],
];
// What the admin is part-way through (Continue Watching) and has finished (Up Next).
const watching = [['Sintel', 0.6], ['Charade', 0.35], ['Metropolis', 0.2]];
const showProgress = [['The Beverly Hillbillies', 2], ['Bonanza', 1]]; // episodes watched, then the next half-way
const watchlist = ['Tears of Steel', 'Nosferatu', 'The Dick Van Dyke Show'];
const profiles = [['Sam', 'teal'], ['Alex', 'amber'], ['Kids', 'rose', 7]];

function run(command, argv, options = {}) {
  const r = spawnSync(command, argv, { cwd: repo, stdio: ['ignore', 'inherit', 'inherit'], ...options });
  if (r.error || r.status !== 0) throw new Error(`${command} ${argv.join(' ')} failed${r.error ? `: ${r.error.message}` : ''}`);
}
const sleep = ms => new Promise(r => setTimeout(r, ms));

const root = await mkdtemp(join(tmpdir(), 'arc-screenshots-'));
let server;
try {
  const media = args.media ? resolve(args.media) : join(root, 'media');
  console.log('Making the library...');
  const clip = join(root, 'clip.mp4');
  // Ten seconds of 1080p so file details read like a normal film; tiny on disk.
  run('ffmpeg', ['-v', 'error', '-y', '-f', 'lavfi', '-i', 'color=c=0x101018:s=1920x1080:r=24:d=10',
    '-f', 'lavfi', '-i', 'anullsrc=r=48000:cl=stereo', '-t', '10', '-c:v', 'libx264', '-preset', 'ultrafast',
    '-tune', 'stillimage', '-c:a', 'aac', '-shortest', clip]);
  for (const m of args.media ? [] : movies) {
    await mkdir(join(media, 'Movies', m), { recursive: true });
    await copyFile(clip, join(media, 'Movies', m, `${m}.mp4`));
  }
  for (const [folder, count] of args.media ? [] : shows) {
    const name = folder.replace(/ \(\d{4}\)$/, '');
    await mkdir(join(media, 'TV', folder, 'Season 01'), { recursive: true });
    for (let e = 1; e <= count; e++) {
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
    if (i > 200 || server.exitCode !== null) throw new Error('Arc did not start');
    await sleep(100);
  }

  const browser = await chromium.launch(args.chrome ? { executablePath: args.chrome } : {});
  const context = await browser.newContext({ baseURL: base, viewport: { width: 1600, height: 1000 }, colorScheme: 'dark' });
  const api = context.request;
  const call = async (method, path, data) => {
    const r = await api.fetch(path, { method, data });
    if (!r.ok()) throw new Error(`${method} ${path}: ${r.status()} ${await r.text()}`);
    return r.status() === 204 ? null : r.json().catch(() => null);
  };

  await call('POST', '/api/auth/setup', { name: 'Jordan', password: 'screenshots-only', local_bypass: true,
    tmdb_api_key: args.tmdb, tvdb_api_key: args.tvdb, region: 'GB' });
  const movieLib = await call('POST', '/api/libraries', { name: 'Movies', kind: 'movies', paths: [join(media, 'Movies')] });
  const tvLib = await call('POST', '/api/libraries', { name: 'TV Shows', kind: 'shows', paths: [join(media, 'TV')],
    source_ordering: args.tvdb ? 'tvdb_aired' : 'tmdb_aired' });

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
    const item = (r.items ?? r).find(i => i.title.toLowerCase().startsWith(title.toLowerCase()));
    if (!item && !quiet) console.warn(`  not found: ${title}`);
    return item;
  };
  for (const [title, part] of watching) {
    const it = await find(movieLib, title);
    const duration = (it?.runtime_min || 100) * 60_000;
    if (it) await call('PUT', `/api/items/${it.id}/progress`, { position_ms: Math.round(duration * part), duration_ms: duration, state: 'stopped' });
  }
  for (const [title, done] of showProgress) {
    const show = await find(tvLib, title);
    if (!show) continue;
    const seasons = await call('GET', `/api/series/${show.id}/seasons`);
    const episodes = (seasons.seasons ?? seasons).flatMap(s => s.episodes).filter(e => e.available !== false);
    for (const e of episodes.slice(0, done)) await call('PUT', `/api/items/${e.id}/played`, { played: true });
    const next = episodes[done];
    if (next) await call('PUT', `/api/items/${next.id}/progress`, { position_ms: 12 * 60_000, duration_ms: 25 * 60_000, state: 'stopped' });
  }
  for (const title of watchlist) {
    const it = (await find(movieLib, title, true)) ?? (await find(tvLib, title));
    if (it) await call('PUT', `/api/items/${it.id}/watchlist`, { on: true });
  }
  for (const [name, color, maxAge] of profiles) await call('POST', '/api/profiles', { name, color, ...(maxAge ? { max_age: maxAge } : {}) });

  console.log('Taking screenshots...');
  await mkdir(out, { recursive: true });
  const page = await context.newPage();
  const shoot = async (path, file, prepare) => {
    await page.goto(path);
    await page.waitForLoadState('networkidle');
    await prepare?.();
    // Off-screen images load lazily and may never finish, so wait a few seconds at most.
    await page.evaluate(() => Promise.race([new Promise(r => setTimeout(r, 5000)),
      Promise.all([...document.images].map(i => i.complete ? null : new Promise(r => { i.onload = i.onerror = r; })))]));
    await sleep(1500); // let entrance animations settle
    await page.screenshot({ path: join(out, file) });
    console.log(`  ${file}`);
  };
  await shoot('/', 'home.png');
  const movie = await find(movieLib, 'Sintel');
  if (movie) await shoot(`/movie/${movie.id}`, 'movie.png');
  const show = await find(tvLib, 'The Beverly Hillbillies');
  if (show) await shoot(`/series/${show.id}`, 'show.png');
  await shoot(`/library/${movieLib.id}`, 'library.png');
  await call('DELETE', '/api/auth/profile');
  await shoot('/', 'profiles.png');
  await browser.close();
  console.log(`Done: ${out}`);
} finally {
  if (server && server.exitCode === null) {
    server.kill();
    await new Promise(r => server.once('exit', r));
  }
  if (args.keep) console.log(`Kept ${root}`);
  else await rm(root, { recursive: true, force: true });
}
