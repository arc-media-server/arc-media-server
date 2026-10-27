import i18n from '../i18n';
import type { ItemSort } from './api';

export function sortLabels(): Record<ItemSort, string> {
  return {
    title: i18n.t('filters:sort.title'),
    added: i18n.t('filters:sort.added'),
    watched: i18n.t('filters:sort.watched'),
    release: i18n.t('filters:sort.release'),
    year: i18n.t('filters:sort.year'),
    rating: i18n.t('filters:sort.rating'),
    imdb: i18n.t('filters:sort.imdb'),
    critics: i18n.t('filters:sort.critics'),
    runtime: i18n.t('filters:sort.runtime'),
    shuffle: i18n.t('filters:sort.shuffle'),
  };
}

export function sortSummary(sort: ItemSort): string {
  const labels: Record<ItemSort, string> = {
    title: i18n.t('filters:sortSummary.title'),
    added: i18n.t('filters:sortSummary.added'),
    watched: i18n.t('filters:sortSummary.watched'),
    release: i18n.t('filters:sortSummary.release'),
    year: i18n.t('filters:sortSummary.year'),
    rating: i18n.t('filters:sortSummary.rating'),
    imdb: i18n.t('filters:sortSummary.imdb'),
    critics: i18n.t('filters:sortSummary.critics'),
    runtime: i18n.t('filters:sortSummary.runtime'),
    shuffle: i18n.t('filters:sortSummary.shuffle'),
  };
  return labels[sort];
}
