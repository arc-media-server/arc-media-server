import type { Library } from './api';

/** Sort only libraries already allowed by the server. Unknown IDs grant no access;
 * new libraries keep their server order after the explicitly arranged ones. */
export function navigationLibraries(libraries: Library[], order: number[] = []): Library[] {
  const rank = new Map(order.map((id, index) => [id, index]));
  return [...libraries].sort((a, b) => (rank.get(a.id) ?? order.length) - (rank.get(b.id) ?? order.length));
}

/** A place in the sidebar's arrangeable list: a library, or the Watchlist (as Plex pins it). */
export type NavEntry = { kind: 'library'; library: Library } | { kind: 'watchlist' };

/** The libraries in the profile's order with the Watchlist after [watchlistAt] of them. */
export function navigationEntries(libraries: Library[], order: number[] = [], watchlistAt = 0): NavEntry[] {
  const entries: NavEntry[] = navigationLibraries(libraries, order).map((library) => ({ kind: 'library', library }));
  entries.splice(Math.max(0, Math.min(watchlistAt, entries.length)), 0, { kind: 'watchlist' });
  return entries;
}

/** An entry's key in a reorderable list. */
export const navEntryKey = (e: NavEntry) => (e.kind === 'watchlist' ? 'watchlist' : `library:${e.library.id}`);
