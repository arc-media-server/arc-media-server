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
    title: "Romance for Valentine's Day",
    from: [2, 1],
    to: [2, 14],
    query: { kind: 'movie', genre: 'Romance', sort: 'shuffle' },
  },
  {
    title: 'Horror for Halloween',
    from: [10, 1],
    to: [10, 31],
    query: { kind: 'movie', genre: 'Horror', sort: 'shuffle' },
  },
  {
    title: 'Christmas Films',
    from: [12, 1],
    to: [12, 31],
    query: { kind: 'movie', about: 'christmas,santa,xmas,north pole,reindeer,elves', sort: 'shuffle' },
  },
];

export function currentSeason(now = new Date()): Season | undefined {
  const md = (now.getMonth() + 1) * 100 + now.getDate();
  return seasons.find((s) => md >= s.from[0] * 100 + s.from[1] && md <= s.to[0] * 100 + s.to[1]);
}

/** A row ready to show: where its items come from and what it's called. */
export type ResolvedRow =
  | { id: string; hidden: boolean; type: 'continue' | 'next-up'; title: string }
  | { id: string; hidden: boolean; type: 'seasonal'; title: string; season?: Season }
  | { id: string; hidden: boolean; type: 'recent'; title: string; library: Library }
  | { id: string; hidden: boolean; type: 'custom'; title: string; library?: Library; filter: RowFilter };

function builtIn(id: string, libraries: Library[]): ResolvedRow | null {
  if (id === 'continue') return { id, hidden: false, type: 'continue', title: 'Continue Watching' };
  if (id === 'next-up') return { id, hidden: false, type: 'next-up', title: 'Up Next' };
  if (id === 'seasonal') {
    const season = currentSeason();
    return { id, hidden: false, type: 'seasonal', title: season?.title ?? 'Seasonal Picks', season };
  }
  if (id.startsWith('recent:')) {
    const library = libraries.find((l) => `recent:${l.id}` === id);
    return library ? { id, hidden: false, type: 'recent', title: `Recently Added in ${library.name}`, library } : null;
  }
  return null;
}

/** The default home page: carry on watching, anything seasonal, then each library's newest. */
export function defaultRowIds(libraries: Library[]) {
  return ['continue', 'next-up', 'seasonal', ...libraries.map((l) => `recent:${l.id}`)];
}

/**
 * The rows to show, in the profile's order. Built-in rows it hasn't placed (e.g. a new library's)
 * go where they'd be by default; rows for things that no longer exist are dropped.
 */
export function resolveRows(saved: HomeRow[], libraries: Library[]): ResolvedRow[] {
  const out: ResolvedRow[] = [];
  const placed = new Set<string>();
  for (const r of saved) {
    if (r.id.startsWith('custom:')) {
      if (!r.filter || !r.title) continue;
      const library = libraries.find((l) => l.id === r.filter!.library_id);
      if (!library) continue;
      out.push({ id: r.id, hidden: !!r.hidden, type: 'custom', title: r.title, library, filter: r.filter });
    } else {
      const b = builtIn(r.id, libraries);
      if (b) out.push({ ...b, hidden: !!r.hidden } as ResolvedRow);
    }
    placed.add(r.id);
  }
  const defaults = defaultRowIds(libraries);
  defaults.forEach((id, i) => {
    if (placed.has(id)) return;
    const b = builtIn(id, libraries)!;
    // After the row that comes before it by default, or first.
    const prev = defaults.slice(0, i).reverse().find((p) => out.some((r) => r.id === p));
    const at = prev ? out.findIndex((r) => r.id === prev) + 1 : 0;
    out.splice(at, 0, b);
  });
  return out;
}

/** What gets saved for a layout. */
export function toSaved(rows: ResolvedRow[]): HomeRow[] {
  return rows.map((r) =>
    r.type === 'custom' ? { id: r.id, hidden: r.hidden || undefined, title: r.title, filter: r.filter } : { id: r.id, hidden: r.hidden || undefined },
  );
}

/** The list query for a custom row. */
export function rowQuery(f: RowFilter, library: Library | undefined, limit = 24): ItemQuery {
  return {
    library_id: f.library_id,
    kind: library?.kind === 'shows' ? 'series' : 'movie',
    genre: f.genre,
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
  if (f.decade) p.set('decade', String(f.decade));
  if (f.resolution) p.set('resolution', f.resolution);
  if (f.unwatched) p.set('unwatched', '1');
  if (f.sort) p.set('sort', f.sort);
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
