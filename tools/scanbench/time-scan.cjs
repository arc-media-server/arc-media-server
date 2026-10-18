// Times a first scan on a fresh Arc: sets it up, adds the movie library, and reports how long until
// 10/50/90/100% of films are matched and until the scan finishes.
// node time-scan.cjs <base url> <library folder> <film count>
const [base, folder, count] = process.argv.slice(2);
const total = Number(count);
let cookie = '';
async function call(method, p, body) {
  const res = await fetch(base + p, { method, headers: { 'Content-Type': 'application/json', Cookie: cookie }, body: body && JSON.stringify(body) });
  const sc = res.headers.get('set-cookie');
  if (sc) cookie = sc.split(';')[0];
  const text = await res.text();
  if (!res.ok) throw new Error(`${method} ${p}: ${res.status} ${text}`);
  return text ? JSON.parse(text) : null;
}
(async () => {
  await call('POST', '/api/auth/setup', { name: 'Bench', password: 'benchpass1', region: 'GB' });
  const t0 = Date.now();
  const lib = await call('POST', '/api/libraries', { name: 'Films', kind: 'movies', paths: [folder] });
  const marks = [0.1, 0.5, 0.9, 1];
  const hit = {};
  let last = '';
  for (;;) {
    await new Promise((r) => setTimeout(r, 2000));
    const st = await call('GET', `/api/libraries/${lib.id}/scan`);
    const items = await call('GET', `/api/items?library_id=${lib.id}&kind=movie&limit=1`);
    // Matched titles: metadata in place (the stand-in gives every film an overview).
    const matched = st.matched ?? 0;
    const s = ((Date.now() - t0) / 1000).toFixed(0);
    const line = `${s}s files=${st.files_seen} probed=${st.files_probed} matched=${matched} listed=${items.total} phase=${st.phase ?? ''} running=${st.running}`;
    if (line.replace(/^\d+s /, '') !== last) console.log(line);
    last = line.replace(/^\d+s /, '');
    for (const m of marks) if (!hit[m] && matched >= Math.ceil(total * m)) (hit[m] = s), console.log(`  ${m * 100}% matched at ${s}s`);
    if (!st.running && st.finished_at) {
      console.log(`scan finished at ${s}s`, JSON.stringify(hit));
      break;
    }
  }
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
