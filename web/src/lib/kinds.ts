import { Clapperboard, Film, Tv, type LucideIcon } from 'lucide-react';
import type { Item, LibraryKind } from '@/lib/api';
import * as fmt from '@/lib/format';

/** The icon for a kind of library. */
export function kindIcon(kind?: LibraryKind): LucideIcon {
  return kind === 'shows' ? Tv : kind === 'videos' ? Clapperboard : Film;
}

/** What a library's titles are called: [one, many]. */
export function kindUnit(kind?: LibraryKind): [string, string] {
  return kind === 'shows' ? ['show', 'shows'] : kind === 'videos' ? ['video', 'videos'] : ['movie', 'movies'];
}

/** A home video (in an Other videos library): never looked up online. */
export function isLocalVideo(item?: Item) {
  return item?.metadata_state === 'locked';
}

/** The line under a card's title: a home video's date, otherwise the year. */
export function cardYear(item: Item) {
  return isLocalVideo(item) && item.release_date ? fmt.date(item.release_date) : item.year || ' ';
}
