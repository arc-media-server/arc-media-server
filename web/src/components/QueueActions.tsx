import { useTranslation } from 'react-i18next';
import { useState } from 'react';
import { api, type Item } from '@/lib/api';
import { nowPlayingFor, usePlaybackQueue } from './Player';

/** Queue choices share the current profile's display order and resume state. */
/** Adding a movie, episode or every available episode of a show to the playback queue. */
export function useQueueActions(item: Item, edition?: string) {
  const { t } = useTranslation('titles');
  const queue = usePlaybackQueue();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [notice, setNotice] = useState('');
  const add = async (next: boolean) => {
    setBusy(true); setError(null);
    try {
      const list = item.kind === 'series'
        ? (await api.seasons(item.id)).seasons.flatMap(s => s.episodes).filter(e => e.available && e.stream_url).map(e => nowPlayingFor({ ...e, series_title: item.title }))
        : [{ ...nowPlayingFor(item), edition }];
      if (!list.length) throw new Error(t('titles:queueActions.noEpisodes'));
      queue.enqueue(list, next);
      setNotice(queue.state ? (next ? t('titles:queueActions.addedNext') : t('titles:queueActions.added') ) : t('titles:queueActions.started'));
    } catch (e) { setError(e); } finally { setBusy(false); }
  };
  return { add, busy, error, notice };
}
