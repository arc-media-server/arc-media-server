import { useState } from 'react';
import { ListVideo, Loader2 } from 'lucide-react';
import { api, type Item } from '@/lib/api';
import { nowPlayingFor, usePlaybackQueue } from './Player';
import { ErrorNotice } from './Page';
import { Button } from './ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from './ui/dropdown-menu';

/** Queue choices share the current profile's display order and resume state. */
export function QueueActions({ item, edition, compact = false }: { item: Item; edition?: string; compact?: boolean }) {
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
      if (!list.length) throw new Error('No available episodes to queue.');
      queue.enqueue(list, next);
      setNotice(queue.state ? (next ? 'Added to play next' : 'Added to queue') : 'Playback started');
    } catch (e) { setError(e); } finally { setBusy(false); }
  };
  return (
    <span className="inline-flex max-w-full flex-wrap items-center gap-2" onClick={e => e.stopPropagation()} onKeyDown={e => e.stopPropagation()}>
      <DropdownMenu>
        <DropdownMenuTrigger asChild><Button variant="secondary" size={compact ? 'icon' : 'lg'} className={compact ? undefined : 'size-12 rounded-2xl px-0'} disabled={busy} title="Playback queue: what plays next, rather than a saved playlist" aria-label={'Queue ' + item.title}>{busy ? <Loader2 className="animate-spin" /> : <ListVideo />}</Button></DropdownMenuTrigger>
        <DropdownMenuContent><DropdownMenuItem onSelect={() => void add(true)}>Play next</DropdownMenuItem><DropdownMenuItem onSelect={() => void add(false)}>Add to queue</DropdownMenuItem></DropdownMenuContent>
      </DropdownMenu>
      <span role="status" className="text-xs text-muted empty:hidden">{notice}</span>
      <ErrorNotice error={error} />
    </span>
  );
}
