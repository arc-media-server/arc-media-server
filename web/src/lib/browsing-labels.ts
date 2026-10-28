import i18n from '../i18n';

/** Existing internal unit identifiers, independent of translated labels. */
export function countUnit(count: number, unit: string): string {
  if (unit === 'show') return i18n.t('library:counts.shows', { count });
  if (unit === 'video') return i18n.t('library:counts.videos', { count });
  if (unit === 'movie') return i18n.t('library:counts.movies', { count });
  return i18n.t('library:counts.titles', { count });
}
