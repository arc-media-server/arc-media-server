// Isolated container acceptance checks. Requires a locally built image and Docker.
// Only named volumes/containers created by this process are removed; no host paths are mounted.
import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { setTimeout as delay } from 'node:timers/promises';

const image = process.argv[2];
const previousImage = process.argv[3];
if (!image) throw new Error('Usage: node tools/docker-smoke.mjs <local-image> [previous-image]');
const prefix = `arc-smoke-${randomUUID()}`;
const volumes = [];
const containers = new Set();
const password = randomUUID(); // Synthetic admin, never an owner credential.

function docker(args, allowFailure = false) {
  const r = spawnSync('docker', args, { encoding: 'utf8', timeout: 120_000, windowsHide: true });
  if (!allowFailure && (r.error || r.status !== 0)) {
    // Do not echo arguments: API bodies may include credentials/cookies.
    throw new Error(`Docker ${args[0]} failed: ${r.error?.message ?? r.stderr}`);
  }
  return r;
}

function volume(suffix) {
  const name = `${prefix}-${suffix}`;
  docker(['volume', 'create', '--label', 'ams.readiness=true', name]);
  volumes.push(name);
  return name;
}

function api(name, path, method = 'GET', data, expected = 200) {
  const args = ['exec', name, 'curl', '-sS', '--max-time', '10',
    '-b', '/tmp/smoke.cookies', '-c', '/tmp/smoke.cookies', '-w', '\n%{http_code}',
    '-X', method, '-H', 'Content-Type: application/json'];
  if (data !== undefined) args.push('--data-binary', JSON.stringify(data));
  args.push(`http://127.0.0.1:8099${path}`);
  const output = docker(args).stdout.trimEnd();
  const split = output.lastIndexOf('\n');
  const status = Number(output.slice(split + 1));
  assert.equal(status, expected, `${method} ${path}: ${output.slice(0, split)}`);
  const body = output.slice(0, split);
  return body ? JSON.parse(body) : undefined;
}

async function healthy(name) {
  const deadline = Date.now() + 90_000;
  while (Date.now() < deadline) {
    const state = JSON.parse(docker(['inspect', name]).stdout)[0].State;
    if (!state.Running) throw new Error('Container exited before becoming healthy');
    if (state.Health?.Status === 'healthy') return;
    await delay(1000);
  }
  throw new Error('Container did not become healthy');
}

// c: the case being checked (Docker --user, PUID/PGID environment, or neither for the default).
function start(name, config, media, c, version = image) {
  containers.add(name);
  docker(['run', '-d', '--name', name, '--network', 'none', '--read-only',
    ...(c.user == null ? [] : ['--user', `${c.user}:${c.user}`]),
    ...Object.entries(c.env ?? {}).flatMap(([k, v]) => ['-e', `${k}=${v}`]),
    '--tmpfs', '/tmp:rw,nosuid,noexec,size=64m',
    '--health-interval', '1s', '--health-start-period', '1s', '--health-timeout', '3s',
    '--mount', `type=volume,source=${config},target=/config${c.user == null ? "" : ",volume-nocopy"}`,
    '--mount', `type=volume,source=${media},target=/media,readonly`, version]);
}

// Arc's own process (PID 1 after the start script hands over), not docker exec's user.
function processOwner(name) {
  return docker(['exec', name, 'stat', '-c', '%u:%g', '/proc/1']).stdout.trim();
}

function remove(name) {
  docker(['rm', '-f', name]);
  containers.delete(name);
}

try {
  // The image starts its script as root and runs Arc as PUID:PGID (linuxserver.io style).
  const imageUser = docker(['image', 'inspect', image, '--format', '{{.Config.User}}']).stdout.trim();
  assert.ok(imageUser === '' || imageUser === 'root' || imageUser === '0', 'The start script needs root only to hand config over');
  const media = volume('media');
  docker(['run', '--rm', '--network', 'none', '--user', '0:0', '--entrypoint', 'sh',
    '--mount', `type=volume,source=${media},target=/media`, image,
    '-c', 'printf "read-only fixture\\n" > /media/keep.txt']);
  const cases = [
    { label: 'image default (PUID:PGID 1000:1000)', uid: 1000, gid: 1000 },
    // An install that ran as root before, as on Unraid: its config is root-owned and handed over.
    { label: 'Unraid PUID:PGID 99:100 over a root-owned config', env: { PUID: '99', PGID: '100' }, uid: 99, gid: 100, rootOwned: true },
    { label: 'PUID=0 (root, asked for)', env: { PUID: '0' }, uid: 0, gid: 0 },
    // Docker's own --user: already unprivileged, so the config must be provisioned for it.
    { label: 'Docker --user 12345', user: 12345, uid: 12345, gid: 12345, provision: true },
  ];
  for (const [index, c] of cases.entries()) {
    const config = volume(`config-${index}`);
    if (c.provision) docker(['run', '--rm', '--network', 'none', '--user', '0:0', '--entrypoint', 'chown',
      '--mount', `type=volume,source=${config},target=/config`, image, `${c.uid}:${c.gid}`, '/config']);
    if (c.rootOwned) docker(['run', '--rm', '--network', 'none', '--user', '0:0', '--entrypoint', 'sh',
      '--mount', `type=volume,source=${config},target=/config`, image,
      '-c', 'mkdir -p /config/from-root && printf old > /config/from-root/old.txt && chown -R 0:0 /config']);
    const name = `${prefix}-${index}`;
    // An upgrade starts from the earlier image as it ran then: as root, unless --user was used.
    start(name, config, media, previousImage ? { user: c.user } : c, previousImage ?? image);
    await healthy(name);
    if (!previousImage) assert.equal(processOwner(name), `${c.uid}:${c.gid}`, `${c.label}: Arc's process user`);
    if (c.rootOwned && !previousImage) {
      assert.equal(docker(['exec', name, 'stat', '-c', '%u:%g', '/config/from-root/old.txt']).stdout.trim(), `${c.uid}:${c.gid}`, 'Root-owned config must be handed to PUID:PGID');
      assert.equal(docker(['exec', name, 'stat', '-c', '%u', '/media/keep.txt']).stdout.trim(), '0', 'Media must never be changed');
    }
    assert.equal(api(name, '/api/auth/status').setup_needed, true);
    docker(['exec', name, 'ffprobe', '-version']);
    docker(['exec', name, 'ffmpeg', '-version']);
    api(name, '/api/auth/setup', 'POST', { name: 'Smoke Admin', password, local_bypass: false }, 201);
    const profile = api(name, '/api/auth/status').profile.id;
    const library = api(name, '/api/libraries', 'POST', { name: 'Read-only media', kind: 'movies', paths: ['/media'] }, 201);
    const scanDeadline = Date.now() + 30_000;
    while (api(name, `/api/libraries/${library.id}/scan`).running) {
      if (Date.now() >= scanDeadline) throw new Error('Empty library scan did not finish');
      await delay(250);
    }
    api(name, '/api/settings', 'PATCH', { region: 'GB', max_transcodes: 1 }, 200);
    let rollback;
    if (previousImage) {
      // Preserve a stopped full directory, including files a DB-only backup omits.
      docker(['exec', name, 'sh', '-c', 'mkdir -p /config/uploads; printf "uploaded fixture" > /config/uploads/smoke.txt']);
      remove(name);
      rollback = volume(`rollback-${index}`);
      docker(['run', '--rm', '--network', 'none', '--user', '0:0', '--entrypoint', 'sh',
        '--mount', `type=volume,source=${config},target=/source,readonly`,
        '--mount', `type=volume,source=${rollback},target=/snapshot`, image,
        '-c', 'cp -a /source/. /snapshot/']);
      start(name, config, media, c);
      await healthy(name);
      assert.equal(processOwner(name), `${c.uid}:${c.gid}`, `${c.label}: Arc's process user after the upgrade`);
      api(name, '/api/auth/login', 'POST', { password }, 204);
      api(name, '/api/auth/profile', 'POST', { id: profile }, 200);
      assert.equal(api(name, '/api/settings').region, 'GB');
      assert.equal(docker(['exec', name, 'cat', '/config/uploads/smoke.txt']).stdout, 'uploaded fixture');
    }
    assert.equal(api(name, '/api/diagnostics').persistent, true, 'Logs must be retained with this container user');
    const boot = api(name, '/api/logs').findLast(l => l.message === 'Arc Media Server started');
    if (process.env.EXPECTED_REVISION) assert.ok(boot?.attrs.includes(`revision=${process.env.EXPECTED_REVISION}`), 'Image must identify the tested revision');
    api(name, '/api/diagnostics', 'PATCH', { detailed: true });
    const backup = api(name, '/api/backups', 'POST', undefined, 201);
    api(name, '/api/settings', 'PATCH', { region: 'US', max_transcodes: 3 }, 200);
    api(name, `/api/backups/${backup.name}/restore`, 'POST', undefined, 204);
    assert.equal(api(name, '/api/settings').region, 'GB');
    assert.notEqual(docker(['exec', name, 'touch', '/media/should-not-exist'], true).status, 0);
    assert.equal(docker(['exec', name, 'cat', '/media/keep.txt']).stdout, 'read-only fixture\n');
    assert.equal(docker(['exec', name, 'stat', '-c', '%u:%g', '/config/ams.db']).stdout.trim(), `${c.uid}:${c.gid}`);
    docker(['restart', name]);
    await healthy(name);
    assert.equal(api(name, '/api/auth/status').setup_needed, false);
    remove(name);
    start(name, config, media, c);
    await healthy(name);
    assert.equal(api(name, '/api/auth/status').setup_needed, false);
    api(name, '/api/auth/login', 'POST', { password }, 204);
    api(name, '/api/auth/profile', 'POST', { id: profile }, 200);
    assert.equal(api(name, '/api/settings').region, 'GB');
    assert.ok(api(name, '/api/backups').backups.some(b => b.name === backup.name));
    assert.equal(api(name, '/api/diagnostics').detailed, false, 'Detailed mode must reset after restarting');
    assert.ok(api(name, '/api/logs').some(l => l.message === 'detailed logging changed'), 'Logs must survive container recreation');
    assert.ok(api(name, '/api/libraries').some(l => l.name === 'Read-only media'));
    remove(name);
    console.log(`PASS: ${c.label}: setup, read-only media, backup/restore, restart and volume persistence`);
    if (rollback) {
      start(name, rollback, media, { user: c.user }, previousImage);
      await healthy(name);
      api(name, '/api/auth/login', 'POST', { password }, 204);
      api(name, '/api/auth/profile', 'POST', { id: profile }, 200);
      assert.equal(api(name, '/api/settings').region, 'GB');
      assert.equal(docker(['exec', name, 'cat', '/config/uploads/smoke.txt']).stdout, 'uploaded fixture');
      assert.ok(api(name, '/api/libraries').some(l => l.name === 'Read-only media'));
      remove(name);
      console.log(`PASS: ${c.label}: earlier-image upgrade and rollback from full stopped-directory snapshot`);
    }
  }
} catch (error) {
  for (const name of containers) {
    const logs = docker(['logs', '--tail', '80', name], true);
    process.stderr.write(logs.stdout ?? '');
    process.stderr.write(logs.stderr ?? '');
  }
  throw error;
} finally {
  for (const name of containers) docker(['rm', '-f', name], true);
  for (const name of volumes) docker(['volume', 'rm', name], true);
}
