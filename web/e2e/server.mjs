// Test-only supervisor. Runs the real embedded app against fresh temporary stores and a local
// metadata service. It never opens the owner's data, config, keys or media.
import { createServer } from 'node:http';
import { mkdtemp, mkdir, copyFile, rm, readdir } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, join, basename, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawn, spawnSync } from 'node:child_process';
import { once } from 'node:events';

const repo = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const root = await mkdtemp(join(tmpdir(), 'arc-e2e-'));
const binary = join(root, process.platform === 'win32' ? 'ams.exe' : 'ams');
const env = Object.fromEntries(Object.entries(process.env).filter(([k]) => !k.toUpperCase().startsWith('AMS_')));
let child;
let generation = 0;
let fixtures;
let stopped = false;
let resetting = false;

function run(command, args, options = {}) {
  const result = spawnSync(command, args, { cwd: repo, env, encoding: 'utf8', ...options });
  if (result.error || result.status !== 0) throw new Error(`${command} failed: ${result.error ?? result.stderr}`);
  return result.stdout;
}

async function ffmpegPath() {
  if (!spawnSync('ffmpeg', ['-version'], { env, stdio: 'ignore' }).error) return 'ffmpeg';
  if (process.platform === 'win32' && env.LOCALAPPDATA) {
    const packages = join(env.LOCALAPPDATA, 'Microsoft', 'WinGet', 'Packages');
    const search = async (folder) => {
      for (const e of await readdir(folder, { withFileTypes: true })) {
        const path = join(folder, e.name);
        if (e.name.toLowerCase() === 'ffmpeg.exe') return path;
        if (e.isDirectory()) { const found = await search(path); if (found) return found; }
      }
    };
    for (const e of await readdir(packages, { withFileTypes: true }).catch(() => [])) {
      if (e.isDirectory() && e.name.startsWith('Gyan.FFmpeg')) {
        const found = await search(join(packages, e.name));
        if (found) return found;
      }
    }
  }
  throw new Error('Browser tests require FFmpeg on PATH (or a Windows winget installation).');
}

async function stopApp() {
  const previous = child;
  child = undefined;
  if (previous && previous.exitCode === null) {
    const exited = once(previous, 'exit');
    previous.kill();
    await exited;
  }
}

async function reset() {
  await stopApp();
  const data = join(root, `data-${++generation}`);
  child = spawn(binary, ['-data', data, '-config', 'none'], {
    cwd: root, env: { ...env, AMS_LISTEN: '127.0.0.1:18199', AMS_TMDB_URL: 'http://127.0.0.1:18200/tmdb',
      AMS_IMDB_RATINGS_URL: 'http://127.0.0.1:18200/no-ratings', AMS_OMDB_URL: 'http://127.0.0.1:18200/no-scores',
      AMS_INTRODB_URL: 'http://127.0.0.1:18200/no-markers', AMS_THEINTRODB_URL: 'http://127.0.0.1:18200/no-markers' },
    stdio: ['ignore', 'ignore', 'pipe'], windowsHide: true,
  });
  let log = '';
  child.stderr.on('data', chunk => { log = (log + chunk).slice(-16_000); });
  const deadline = Date.now() + 20_000;
  while (Date.now() < deadline) {
    if (child.exitCode !== null) throw new Error(`Arc exited: ${log}`);
    try { if ((await fetch('http://127.0.0.1:18199/api/health')).ok) return; } catch {}
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  throw new Error(`Arc did not become ready: ${log}`);
}

const movie = (id, title) => ({ id, title, original_title: title, original_language: 'en', release_date: '2020-01-01',
  overview: 'A locally generated movie for browser regression tests.', runtime: 2, genres: [],
  release_dates: { results: [{ iso_3166_1: 'GB', release_dates: [{ certification: 'PG', type: 3 }] }] } });
const episodes = [1, 2].map(n => ({ id: 3000 + n, name: n === 1 ? 'First Chapter' : 'Second Chapter',
  overview: `Chapter ${n} of the regression fixture.`, season_number: 1, episode_number: n, runtime: 2,
  air_date: `2020-01-0${n}` }));
const show = { id: 303, name: 'Arc Show', original_name: 'Arc Show', original_language: 'en',
  overview: 'A local series with an alternative display order.', first_air_date: '2020-01-01',
  seasons: [{ season_number: 1, name: 'Season 1' }], episode_groups: { results: [{ id: 'reverse', name: 'Story Order', type: 5 }] },
  'season/1': { overview: 'Fixture season.', episodes }, content_ratings: { results: [{ iso_3166_1: 'GB', rating: 'PG' }] } };

function metadata(url) {
  const p = url.pathname.slice('/tmdb'.length);
  const query = (url.searchParams.get('query') ?? '').toLowerCase();
  if (p === '/configuration') return { images: {} };
  if (p === '/search/movie') return { results: query === 'arc movie' ? [movie(101, 'Arc Movie')] : query === 'resolved movie' ? [movie(202, 'Resolved Movie')] : [] };
  if (p === '/movie/404') return movie(404, 'Remux Movie');
  if (p === '/movie/505') return movie(505, 'Transcode Movie');
  if (p === '/movie/101') return movie(101, 'Arc Movie');
  if (p === '/movie/202') return movie(202, 'Resolved Movie');
  if (p === '/search/tv') return { results: query === 'arc show' ? [show] : [] };
  if (p === '/tv/303') return show;
  if (p === '/tv/episode_group/reverse') return { id: 'reverse', name: 'Story Order', type: 5,
    groups: [{ order: 1, name: 'Story Season', episodes: [...episodes].reverse().map((e, order) => ({ ...e, order })) }] };
  if (p.startsWith('/find/')) return { movie_results: [], tv_results: [] };
  return null;
}

const server = createServer(async (req, res) => {
  const url = new URL(req.url, 'http://127.0.0.1');
  const json = (status, body) => { res.writeHead(status, { 'Content-Type': 'application/json' }); res.end(JSON.stringify(body)); };
  try {
    if (url.pathname === '/health') return json(200, { ready: true });
    if (url.pathname === '/fixtures') return json(200, fixtures);
    if (url.pathname === '/shutdown' && req.method === 'POST') { await cleanup(); return json(200, { stopped: true }); }
    if (url.pathname === '/reset' && req.method === 'POST') {
      if (resetting) return json(409, { error: 'reset already running' });
      resetting = true;
      try { await reset(); return json(200, fixtures); } finally { resetting = false; }
    }
    const body = metadata(url);
    json(body ? 200 : 404, body ?? { error: `No test fixture for ${url.pathname}` });
  } catch (err) { console.error(err); json(500, { error: String(err) }); }
});

async function cleanup() {
  if (stopped) return;
  stopped = true;
  await stopApp();
  server.close();
  // Only remove the exact directory this process created, never a configurable data/media path.
  if (dirname(root) === tmpdir() && basename(root).startsWith('arc-e2e-')) await rm(root, { recursive: true, force: true });
}
for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, () => { void cleanup().then(() => process.exit(0)); });

try {
  const ffmpeg = await ffmpegPath();
  if (ffmpeg !== 'ffmpeg') env.PATH = dirname(ffmpeg) + ';' + (env.PATH ?? env.Path ?? '');
  run('go', ['build', '-o', binary, './cmd/ams']);
  const media = join(root, 'media');
  fixtures = { movies: join(media, 'Movies'), shows: join(media, 'TV'), empty: join(media, 'Empty'), playback: join(media, 'Playback') };
  for (const p of Object.values(fixtures)) await mkdir(p, { recursive: true });
  const sample = join(root, 'sample.mp4');
  run(ffmpeg, ['-v', 'error', '-y', '-f', 'lavfi', '-i', 'testsrc2=duration=120:size=160x90:rate=10',
    '-f', 'lavfi', '-i', 'sine=duration=120', '-c:v', 'libx264', '-preset', 'ultrafast', '-pix_fmt', 'yuv420p',
    '-c:a', 'aac', '-movflags', '+faststart', '-shortest', sample]);
  for (const name of ['Arc Movie (2020).mp4', 'Unmatched Mystery (2020).mp4']) await copyFile(sample, join(fixtures.movies, name));
  run(ffmpeg, ['-v', 'error', '-y', '-i', sample, '-c:v', 'copy', '-c:a', 'ac3',
    join(fixtures.playback, 'Remux Movie (2020) {tmdb-404}.mkv')]);
  run(ffmpeg, ['-v', 'error', '-y', '-i', sample, '-c:v', 'mpeg2video', '-q:v', '4', '-c:a', 'ac3',
    join(fixtures.playback, 'Transcode Movie (2020) {tmdb-505}.mkv')]);
  const season = join(fixtures.shows, 'Arc Show (2020)', 'Season 01');
  await mkdir(season, { recursive: true });
  for (const n of ['01', '02', '99']) await copyFile(sample, join(season, `Arc Show - S01E${n}.mp4`));
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(18200, '127.0.0.1', resolve); });
  console.log('Isolated browser fixture server ready.');
} catch (err) { console.error(err); await cleanup(); process.exit(1); }
