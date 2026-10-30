import i18n from '@/i18n';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api, type HomeRow, type Item, type ItemQuery, type Library, type Prefs, type RowFilter } from '@/lib/api';

/** The current profile's preferences (home layout, spoilers). */
export function usePrefs() {
  return useQuery({ queryKey: ['prefs'], queryFn: api.prefs, staleTime: Infinity });
}

/** Saves preferences, showing the change straight away. */
export function useUpdatePrefs() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: api.updatePrefs,
    onMutate: (patch) => {
      const before = qc.getQueryData<Prefs>(['prefs']);
      if (before) qc.setQueryData<Prefs>(['prefs'], { ...before, ...patch });
      return before;
    },
    onError: (_e, _patch, before) => before && qc.setQueryData(['prefs'], before),
    onSuccess: (data) => qc.setQueryData(['prefs'], data),
  });
}

/** A seasonal collection, shown on the home page during its dates. */
export interface Season {
  title: string;
  /** Month (1–12) and day the season starts and ends, inclusive. */
  from: [number, number];
  to: [number, number];
  query: ItemQuery;
}

// Built on the same filters as browsing and custom rows.
export const seasons: Season[] = [
  {
    get title() { return i18n.t('home:seasonal.romance'); },
    from: [2, 1],
    to: [2, 14],
    query: { kind: 'movie', genre: 'Romance', sort: 'shuffle' },
  },
  {
    get title() { return i18n.t('home:seasonal.horror'); },
    from: [10, 1],
    to: [10, 31],
    query: { kind: 'movie', genre: 'Horror', sort: 'shuffle' },
  },
  {
    get title() { return i18n.t('home:seasonal.christmas'); },
    from: [12, 1],
    to: [12, 31],
    query: { kind: 'movie', about: 'christmas,santa,xmas,north pole,reindeer,elves', sort: 'shuffle' },
  },
];

export function currentSeason(now = new Date()): Season | undefined {
  const md = (now.getMonth() + 1) * 100 + now.getDate();
  return seasons.find((s) => md >= s.from[0] * 100 + s.from[1] && md <= s.to[0] * 100 + s.to[1]);
}

/** How a row looks: its cards' style and, for a TV library's Recently Added, whether new episodes are grouped by season. */
export interface RowLook {
  /** arc is shown as "Backdrops": squarer cards with the title's background art and logo, as on Apple TV. */
  style: 'poster' | 'thumb' | 'arc';
  showsOnly: boolean;
  /** Episodes and seasons on poster rows: the show's poster or the season's. */
  posters: 'series' | 'season';
}

/** A row ready to show: where its items come from and what it's called. */
export type ResolvedRow = RowLook &
  (
    | { id: string; hidden: boolean; type: 'continue' | 'next-up' | 'watchlist'; title: string }
    | { id: string; hidden: boolean; type: 'seasonal'; title: string; season?: Season }
    | { id: string; hidden: boolean; type: 'recent'; title: string; library: Library }
    | { id: string; hidden: boolean; type: 'custom'; title: string; library?: Library; filter: RowFilter }
    | { id: string; hidden: boolean; type: 'collection' | 'playlist'; title: string; ref: string }
  );

/** Carrying on watching uses landscape thumbnails, everything else posters, as in Plex. */
export function defaultStyle(type: ResolvedRow['type'], library?: Library): RowLook['style'] {
  return type === 'continue' || type === 'next-up' || library?.kind === 'videos' ? 'thumb' : 'poster';
}

/** Recently Added shows a new season with its own poster; carrying on watching shows the show's. */
export function defaultPosters(type: ResolvedRow['type']): RowLook['posters'] {
  return type === 'recent' ? 'season' : 'series';
}

/** Whether a row can show episodes or seasons, so season posters mean something. */
export function hasEpisodes(r: ResolvedRow) {
  return r.type === 'continue' || r.type === 'next-up' || r.type === 'playlist' || (r.type === 'recent' && r.library.kind === 'shows');
}

/** The library a Recently Added or filter row is for. */
function rowLibrary(r: ResolvedRow) {
  return r.type === 'recent' || r.type === 'custom' ? r.library : undefined;
}

/**
 * Whether the profile chooses the row's card style: only the rows carrying on watching (owner,
 * 30 October, as Plex). Every other row keeps its usual look; a style saved for one earlier is ignored.
 */
export function choosesLook(type: ResolvedRow['type']) {
  return type === 'continue' || type === 'next-up';
}

function look(type: ResolvedRow['type'], saved?: HomeRow, library?: Library): RowLook {
  const chosen = choosesLook(type) ? saved : undefined;
  return { style: chosen?.style ?? defaultStyle(type, library), showsOnly: !!saved?.shows_only, posters: chosen?.posters ?? defaultPosters(type) };
}

function builtIn(id: string, libraries: Library[], saved?: HomeRow): ResolvedRow | null {
  if (id === 'continue') return { id, hidden: false, type: 'continue', title: i18n.t('home:rows.continue'), ...look('continue', saved) };
  if (id === 'next-up') return { id, hidden: false, type: 'next-up', title: i18n.t('home:rows.next'), ...look('next-up', saved) };
  if (id === 'watchlist') return { id, hidden: false, type: 'watchlist', title: i18n.t('common:navigation.watchlist'), ...look('watchlist', saved) };
  if (id === 'seasonal') {
    const season = currentSeason();
    return { id, hidden: false, type: 'seasonal', title: season?.title ?? i18n.t('home:rows.seasonal'), season, ...look('seasonal', saved) };
  }
  if (id.startsWith('recent:')) {
    const library = libraries.find((l) => `recent:${l.id}` === id);
    return library ? { id, hidden: false, type: 'recent', title: i18n.t('home:rows.recent', { name: library.name }), library, ...look('recent', saved, library) } : null;
  }
  return null;
}

/** The default home page: carry on watching, anything seasonal, then each library's newest. */
export function defaultRowIds(libraries: Library[]) {
  return ['continue', 'next-up', 'watchlist', 'seasonal', ...libraries.map((l) => `recent:${l.id}`)];
}

/**
 * The rows to show. Their order follows the profile's library order, as Plex's Home follows its
 * pinned libraries (owner, 30 October): carrying on watching, the watchlist and anything seasonal
 * first, then each library's newest followed by its own filter rows, then collections and
 * playlists. [libraries] must be in the profile's order (navigationLibraries). The saved layout
 * says which rows exist, which are hidden and how the carrying-on rows look; rows for things that
 * no longer exist are dropped.
 */
export function resolveRows(saved: HomeRow[], libraries: Library[]): ResolvedRow[] {
  const byId = new Map(saved.map((r) => [r.id, r]));
  const rows: ResolvedRow[] = [];
  for (const id of defaultRowIds(libraries)) {
    const b = builtIn(id, libraries, byId.get(id));
    if (b) rows.push({ ...b, hidden: !!byId.get(id)?.hidden } as ResolvedRow);
  }
  for (const r of saved) {
    if (r.id.startsWith('collection:') || r.id.startsWith('playlist:')) {
      const type = r.id.startsWith('collection:') ? 'collection' : 'playlist';
      rows.push({ id: r.id, hidden: !!r.hidden, type, title: r.title || (type === 'collection' ? i18n.t('home:rowEditor.collection') : i18n.t('playlists:kind.manual')), ref: r.id.slice(type.length + 1), ...look(type, r) });
    } else if (r.id.startsWith('custom:')) {
      if (!r.filter || !r.title) continue;
      const library = libraries.find((l) => l.id === r.filter!.library_id);
      if (!library) continue;
      rows.push({ id: r.id, hidden: !!r.hidden, type: 'custom', title: r.title, library, filter: r.filter, ...look('custom', r, library) });
    }
  }
  return arrangeRows(rows, libraries);
}

/** Puts rows in Home's order for [libraries] (in the profile's order), keeping ties as they were. */
export function arrangeRows(rows: ResolvedRow[], libraries: Library[]): ResolvedRow[] {
  const top = ['continue', 'next-up', 'watchlist', 'seasonal'];
  const libraryRank = new Map(libraries.map((l, i) => [l.id, i]));
  const rank = (r: ResolvedRow): [number, number] => {
    if (top.includes(r.type)) return [0, top.indexOf(r.type)];
    const lib = r.type === 'recent' ? r.library : r.type === 'custom' ? r.library : undefined;
    if (lib) return [1 + (libraryRank.get(lib.id) ?? libraries.length), r.type === 'recent' ? 0 : 1];
    return [2 + libraries.length, 0];
  };
  return rows
    .map((r, i) => ({ r, i, k: rank(r) }))
    .sort((a, b) => a.k[0] - b.k[0] || a.k[1] - b.k[1] || a.i - b.i)
    .map((x) => x.r);
}

/** The rows a layout shows: without Up Next when it's part of Continue Watching. */
export function visibleRows(rows: ResolvedRow[], splitUpNext: boolean) {
  return splitUpNext ? rows : rows.filter((r) => r.type !== 'next-up');
}

/** Whether a row looks the way it does by default. */
export function isDefaultLook(r: ResolvedRow) {
  return r.style === defaultStyle(r.type, rowLibrary(r)) && !r.showsOnly && r.posters === defaultPosters(r.type);
}

/** What gets saved for a layout. */
export function toSaved(rows: ResolvedRow[]): HomeRow[] {
  return rows.map((r) => ({
    id: r.id,
    hidden: r.hidden || undefined,
    ...(r.type === 'custom' && { title: r.title, filter: r.filter }),
    ...((r.type === 'collection' || r.type === 'playlist') && { title: r.title }),
    style: r.style === defaultStyle(r.type, rowLibrary(r)) ? undefined : r.style,
    shows_only: r.showsOnly || undefined,
    posters: r.posters === defaultPosters(r.type) ? undefined : r.posters,
  }));
}

/** The list query for a custom row. */
export function rowQuery(f: RowFilter, library: Library | undefined, limit = 24): ItemQuery {
  return {
    library_id: f.library_id,
    kind: library?.kind === 'shows' ? 'series' : 'movie',
    genre: f.genre,
    language: f.language,
    label: f.label,
    decade: f.decade,
    resolution: f.resolution,
    unwatched: f.unwatched,
    added_within: f.added_within,
    min_runtime: f.min_runtime,
    max_runtime: f.max_runtime,
    min_rating: f.min_rating,
    sort: f.sort,
    limit,
  };
}

/** A link to the library page showing the same thing (for filters it understands). */
export function rowHref(f: RowFilter) {
  const p = new URLSearchParams();
  if (f.genre) p.set('genre', f.genre);
  if (f.language) p.set('language', f.language);
  if (f.label) p.set('label', f.label);
  if (f.decade) p.set('decade', String(f.decade));
  if (f.resolution) p.set('resolution', f.resolution);
  if (f.unwatched) p.set('unwatched', '1');
  if (f.sort) p.set('sort', f.sort);
  for (const key of ['added_within', 'min_runtime', 'max_runtime', 'min_rating'] as const) if (f[key]) p.set(key, String(f[key]));
  const s = p.toString();
  return `/library/${f.library_id}${s ? `?${s}` : ''}`;
}

/**
 * With spoiler protection on, whether an episode's still and description should be hidden:
 * episodes the profile hasn't started yet.
 */
export function useSpoilerGuard() {
  const { data } = usePrefs();
  const on = !!data?.hide_spoilers;
  return (item: Item) => on && item.kind === 'episode' && !item.user_data?.played && !item.user_data?.position_ms;
}

/** Built-in labels follow the active language; user/provider titles remain verbatim. */
export function rowTitle(row: ResolvedRow): string {
  if (row.type === 'custom' || row.type === 'collection' || row.type === 'playlist') return row.title;
  return builtIn(row.id, row.type === 'recent' ? [row.library] : [])?.title ?? row.title;
}
