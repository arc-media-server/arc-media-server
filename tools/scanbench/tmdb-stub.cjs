// A stand-in TMDB for timing scans: every request is answered after DELAY ms (a real round trip
// is roughly 150-300 ms), and every search finds the film it was asked for.
const http = require('http');
const DELAY = Number(process.env.DELAY || 200);
let requests = 0;
const idOf = (s) => {
  let h = 0;
  for (const c of s) h = (h * 31 + c.charCodeAt(0)) >>> 0;
  return (h % 9000000) + 1000;
};
const titles = new Map();
http
  .createServer((req, res) => {
    requests++;
    const u = new URL(req.url, 'http://x');
    setTimeout(() => {
      res.setHeader('Content-Type', 'application/json');
      if (u.pathname === '/__count') return res.end(JSON.stringify({ requests }));
      if (u.pathname === '/search/movie') {
        const q = u.searchParams.get('query') || '';
        const y = u.searchParams.get('year') || '2000';
        const id = idOf(q + y);
        titles.set(id, { q, y });
        return res.end(JSON.stringify({ results: [{ id, title: q, original_title: q, release_date: `${y}-06-01`, overview: 'A film.' }] }));
      }
      const m = u.pathname.match(/^\/movie\/(\d+)$/);
      if (m) {
        const id = Number(m[1]);
        const t = titles.get(id) || { q: 'Film ' + id, y: '2000' };
        return res.end(
          JSON.stringify({
            id, imdb_id: 'tt' + id, title: t.q, original_title: t.q, original_language: 'en', overview: 'A film.', release_date: `${t.y}-06-01`,
            runtime: 100, vote_average: 7, genres: [{ id: 1, name: 'Drama' }], production_companies: [],
            credits: { cast: [{ id: id + 1, name: 'An Actor', character: 'Someone', order: 0 }], crew: [] },
            release_dates: { results: [] }, images: { logos: [] }, recommendations: { results: [] }, alternative_titles: { titles: [] }, keywords: { keywords: [] },
          }),
        );
      }
      res.statusCode = 404;
      res.end('{"status_message":"not found"}');
    }, DELAY);
  })
  .listen(8305, '127.0.0.1', () => console.log('tmdb stub on 8305, delay', DELAY));
