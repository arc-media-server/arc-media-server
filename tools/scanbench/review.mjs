// End-to-end movie-import measurements using only generated clips, fresh temporary stores
// and a local TMDB stand-in. Never point this at an existing Arc or real media directory.
import { createServer } from 'node:http';
import { mkdtemp, mkdir, copyFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, join, basename, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawn, spawnSync } from 'node:child_process';
import { once } from 'node:events';

const repo = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const root = await mkdtemp(join(tmpdir(), 'arc-scan-review-'));
const count = 120;
const providerDelayMS = 200;
const env = Object.fromEntries(Object.entries(process.env).filter(([k]) => !k.toUpperCase().startsWith('AMS_')));
let child;
let requests = {};
const titles = new Map();
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
const stub = createServer(async (req, res) => {
  const u = new URL(req.url, 'http://localhost');
  const p = u.pathname;
  if (p === '/configuration' || p === '/search/movie' || /^\/movie\/\d+$/.test(p)) {
    requests[p === '/search/movie' ? 'search' : p === '/configuration' ? 'configuration' : 'details'] = (requests[p === '/search/movie' ? 'search' : p === '/configuration' ? 'configuration' : 'details'] ?? 0) + 1;
    await pause(providerDelayMS);
  }
  let data;
  if (p === '/configuration') data = { images: {} };
  if (p === '/search/movie') {
    const title = u.searchParams.get('query');
    const id = Number(title.match(/\d+/)?.[0] ?? 0) + 1000;
    titles.set(id, title);
    data = { results: [{ id, title, original_title: title, release_date: '2000-01-01' }] };
  }
  if (/^\/movie\/\d+$/.test(p)) {
    const id = Number(p.split('/').at(-1));
    data = { id, title: titles.get(id) ?? `Review ${id - 1000}`, original_title: titles.get(id) ?? `Review ${id - 1000}`,
      overview: 'Local timing fixture', release_date: '2000-01-01', runtime: 1, genres: [], credits: { cast: [], crew: [] },
      images: { logos: [] }, release_dates: { results: [] }, recommendations: { results: [] } };
  }
  res.writeHead(data ? 200 : 404, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(data ?? { error: 'No external services in scan review' }));
});

async function stop() {
  if (child && child.exitCode === null) {
    const exited = once(child, 'exit');
    child.kill();
    await exited;
  }
  child = undefined;
}

try {
  const binary = join(root, process.platform === 'win32' ? 'ams.exe' : 'ams');
  for (const [cmd, args] of [
    ['go', ['build', '-o', binary, './cmd/ams']],
    ['ffmpeg', ['-v', 'error', '-f', 'lavfi', '-i', 'color=duration=1:size=160x90:rate=10', '-c:v', 'libx264', '-pix_fmt', 'yuv420p', join(root, 'clip.mp4')]],
  ]) {
    const run = spawnSync(cmd, args, { cwd: repo, env, encoding: 'utf8', windowsHide: true });
    if (run.error || run.status !== 0) throw new Error(`${cmd}: ${run.error ?? run.stderr}`);
  }
  const media = join(root, 'media');
  for (let i = 0; i < count; i++) {
    const title = `Review ${String(i).padStart(4, '0')} (2000)`;
    const folder = join(media, title);
    await mkdir(folder, { recursive: true });
    await copyFile(join(root, 'clip.mp4'), join(folder, `${title}.mp4`));
  }
  await new Promise(resolve => stub.listen(0, '127.0.0.1', resolve));
  const service = `http://127.0.0.1:${stub.address().port}`;
  const results = [];
  for (const preview of [false, true]) {
    // Ask the OS for a currently free loopback port; never connect to a pre-existing Arc.
    const reservation = createServer();
    await new Promise(resolve => reservation.listen(0, '127.0.0.1', resolve));
    const port = reservation.address().port;
    await new Promise(resolve => reservation.close(resolve));
    const base = `http://127.0.0.1:${port}`;
    const data = join(root, preview ? 'preview-data' : 'direct-data');
    child = spawn(binary, ['-data', data, '-config', 'none'], { cwd: root, env: { ...env,
      AMS_LISTEN: `127.0.0.1:${port}`, AMS_TMDB_URL: service, AMS_TMDB_API_KEY: 'local-fixture', AMS_PROBE_DELAY_MS: '300',
      AMS_IMDB_RATINGS_URL: `${service}/no-ratings`, AMS_OMDB_URL: `${service}/no-scores`,
      AMS_INTRODB_URL: `${service}/no-markers`, AMS_THEINTRODB_URL: `${service}/no-markers`,
    }, stdio: ['ignore', 'ignore', 'pipe'], windowsHide: true });
    let log = '';
    child.stderr.on('data', c => { log = (log + c).slice(-8000); });
    for (let i = 0; ; i++) {
      if (child.exitCode !== null || i > 200) throw new Error(`Arc not ready: ${log}`);
      try { if ((await fetch(`${base}/api/health`)).ok) break; } catch {}
      await pause(100);
    }
    let cookie = '';
    const call = async (method, path, body) => {
      const r = await fetch(base + path, { method, headers: { 'Content-Type': 'application/json', Cookie: cookie }, body: body && JSON.stringify(body) });
      const c = r.headers.get('set-cookie');
      if (c) cookie = c.split(';')[0];
      const text = await r.text();
      if (!r.ok) throw new Error(`${path}: ${r.status} ${text}`);
      return text ? JSON.parse(text) : null;
    };
    await call('POST', '/api/auth/setup', { name: 'Scan review', password: 'Review-fixture-123', region: 'GB' });
    requests = {};
    const started = performance.now();
    let previewSeconds = 0;
    if (preview) {
      const p = await call('POST', '/api/library-previews', { kind: 'movies', paths: [media] });
      while ((await call('GET', `/api/library-previews/${p.id}`)).stage !== 'done') await pause(100);
      previewSeconds = (performance.now() - started) / 1000;
    }
    const lib = await call('POST', '/api/libraries', { name: 'Review', kind: 'movies', paths: [media] });
    let firstMatched, allMatched;
    const deadline = performance.now() + 180_000;
    for (;;) {
      if (performance.now() > deadline) throw new Error('Scan exceeded review deadline');
      const s = await call('GET', `/api/libraries/${lib.id}/scan`);
      const seconds = (performance.now() - started) / 1000;
      if (s.matched > 0 && firstMatched === undefined) firstMatched = seconds;
      if (s.matched === count && allMatched === undefined) allMatched = seconds;
      if (!s.running && s.finished_at) {
        if (s.error || s.matched !== count || s.files_probed !== count) throw new Error(`Incomplete scan: ${JSON.stringify(s)}`);
        const result = { mode: preview ? 'preview-then-add' : 'direct-add', count, providerDelayMS, probeDelayMS: 300,
          previewSeconds, firstMatched, allMatched, finishedSeconds: seconds, providerRequests: { ...requests } };
        results.push(result);
        console.log(JSON.stringify(result));
        break;
      }
      await pause(100);
    }
    await stop();
  }
  console.log(JSON.stringify({ results, limits: 'Generated 1s MP4s, local Windows disk, local delayed provider. Not Plex, live-provider or Unraid measurements.' }));
} finally {
  await stop();
  await new Promise(resolve => stub.close(resolve));
  if (dirname(root) === tmpdir() && basename(root).startsWith('arc-scan-review-')) await rm(root, { recursive: true, force: true });
}
