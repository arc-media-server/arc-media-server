import type { ItemSort } from './api';

/** Durable browsing controls only: no view tabs, letter jumps, paging or selection. */
export const browseFilterKeys = [
  'genre', 'language', 'label', 'decade', 'resolution', 'unwatched', 'unmatched',
  'added_within', 'min_runtime', 'max_runtime', 'min_rating',
] as const;
const sorts: ItemSort[] = ['title', 'added', 'year', 'release', 'rating', 'critics', 'runtime', 'watched', 'shuffle'];

/** Accept only controls understood by the library; old/corrupt storage stays harmless. */
export function persistentBrowseParams(params: URLSearchParams): URLSearchParams {
  const saved = new URLSearchParams();
  const sort = params.get('sort') as ItemSort;
  saved.set('sort', sorts.includes(sort) ? sort : 'title');
  if (params.get('reverse') === '1') saved.set('reverse', '1');
  for (const key of browseFilterKeys) {
    const value = params.get(key);
    if (!value) continue;
    if (key === 'unwatched' || key === 'unmatched') {
      if (value === '1') saved.set(key, value);
    } else if (key === 'resolution') {
      if (['4k', '1080', '720', 'sd'].includes(value)) saved.set(key, value);
    } else if (key === 'genre' || key === 'language' || key === 'label') {
      if (value.length <= 200) saved.set(key, value);
    } else {
      const number = Number(value);
      if (Number.isFinite(number) && number > 0) saved.set(key, String(number));
    }
  }
  return saved;
}

export function browseStorageKey(profile: string, library: string): string {
  return `arc-browse:${profile}:${library}`;
}

export function readBrowseState(profile: string, library: string): URLSearchParams | undefined {
  try {
    const saved = localStorage.getItem(browseStorageKey(profile, library));
    return saved ? persistentBrowseParams(new URLSearchParams(saved)) : undefined;
  } catch { return undefined; /* Storage may be unavailable in private browsing. */ }
}

export function rememberBrowseState(profile: string, library: string, params: URLSearchParams): void {
  try { localStorage.setItem(browseStorageKey(profile, library), persistentBrowseParams(params).toString()); }
  catch { /* Remembering controls is optional; the URL remains usable. */ }
}
