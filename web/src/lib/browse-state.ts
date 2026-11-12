import type { ItemSort } from './api';

import { encodeItemFilters, readItemFilters } from './item-filters';
export { browseFilterKeys } from './item-filters';
const sorts: ItemSort[] = ['title', 'added', 'year', 'release', 'rating', 'critics', 'runtime', 'watched', 'shuffle'];

/** Accept only controls understood by the library; old/corrupt storage stays harmless. */
export function persistentBrowseParams(params: URLSearchParams): URLSearchParams {
  const saved = encodeItemFilters(readItemFilters(params));
  const sort = params.get('sort') as ItemSort;
  saved.set('sort', sorts.includes(sort) ? sort : 'title');
  if (params.get('reverse') === '1') saved.set('reverse', '1');
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
