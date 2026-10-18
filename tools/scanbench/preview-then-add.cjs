// Previews the library, then adds it, counting TMDB requests for each and timing the scan.
// node preview-then-add.cjs <base url> <library folder> <film count>
const [base, folder, count] = process.argv.slice(2);
let cookie = '';
async function call(method, p, body) {
  const res = await fetch(base + p, { method, headers: { 'Content-Type': 'application/json', Cookie: cookie }, body: body && JSON.stringify(body) });
  const sc = res.headers.get('set-cookie');
  if (sc) cookie = sc.split(';')[0];
  const text = await res.text();
  if (!res.ok) throw new Error(`${method} ${p}: ${res.status} ${text}`);
  return text ? JSON.parse(text) : null;
}
const tmdbCount = async () => (await (await fetch('http://127.0.0.1:8305/__count')).json()).requests;
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
(async () => {
  await call('POST', '/api/auth/setup', { name: 'Bench', password: 'benchpass1', region: 'GB' });
  let c0 = await tmdbCount();
  let t = Date.now();
  const { id } = await call('POST', '/api/library-previews', { kind: 'movies', paths: [folder] });
  for (;;) {
    await sleep(1000);
    const p = await call('GET', `/api/library-previews/${id}`);
    if (p.stage === 'done') {
      console.log(`preview: ${p.items.length} films, ${p.items.filter((i) => i.status === 'identified').length} identified, ${((Date.now() - t) / 1000).toFixed(0)}s, ${(await tmdbCount()) - c0 - 1} TMDB requests`);
      break;
    }
  }
  c0 = await tmdbCount();
  t = Date.now();
  const lib = await call('POST', '/api/libraries', { name: 'Films', kind: 'movies', paths: [folder] });
  let all = 0;
  for (;;) {
    await sleep(1000);
    const st = await call('GET', `/api/libraries/${lib.id}/scan`);
    if (!all && st.matched >= Number(count)) all = ((Date.now() - t) / 1000).toFixed(0);
    if (!st.running && st.finished_at) break;
  }
  console.log(`scan: all matched at ${all}s, finished at ${((Date.now() - t) / 1000).toFixed(0)}s, ${(await tmdbCount()) - c0 - 1} TMDB requests`);
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
