import { useTranslation } from 'react-i18next';
import { Check } from 'lucide-react';
import type { Item } from '@/lib/api';
import { cn } from '@/lib/utils';

/** How far through an item the profile is, 0–1, or 0 when not started. */
export function progressOf(item: Item): number {
  const pos = item.user_data?.position_ms ?? 0;
  const dur = (item.runtime_min ?? 0) * 60_000 || item.files?.[0]?.duration_ms || 0;
  return pos && dur ? Math.min(1, pos / dur) : 0;
}

/** A thin bar along the bottom of artwork showing progress. */
export function ProgressBar({ value, className }: { value: number; className?: string }) {
  if (value <= 0) return null;
  return (
    <div className={cn('absolute inset-x-2 bottom-2 h-1 overflow-hidden rounded-full bg-black/50 backdrop-blur', className)}>
      <div className="h-full rounded-full bg-accent" style={{ width: `${Math.max(4, value * 100)}%` }} />
    </div>
  );
}

/**
 * The corner badge on a poster: a tick when watched, or for a show the number of unwatched episodes.
 */
export function WatchBadge({ item, className }: { item: Item; className?: string }) {
  const { t } = useTranslation('titles');
  const watched = item.kind === 'series' ? item.unwatched_count === 0 : item.user_data?.played;
  const count = item.kind === 'series' ? (item.unwatched_count ?? 0) : 0;
  if (!watched && count === 0) return null;
  return (
    <span
      className={cn(
        'absolute right-1.5 top-1.5 flex h-6 min-w-6 items-center justify-center rounded-full px-1.5 text-[11px] font-bold shadow-lg',
        watched ? 'bg-black/55 text-white backdrop-blur' : 'bg-accent-strong text-accent-foreground',
        className,
      )}
      title={watched ? t('titles:watchState.watched') : t('titles:watchState.unwatched', { count })}
    >
      {watched ? <Check className="size-3.5" strokeWidth={3} /> : count}
    </span>
  );
}

/** A number in the watch badge's place and style, such as a Recently Added season's new episodes. */
export function CountBadge({ count, label, className }: { count: number; label: string; className?: string }) {
  return (
    <span
      className={cn('absolute right-1.5 top-1.5 flex h-6 min-w-6 items-center justify-center rounded-full bg-accent-strong px-1.5 text-[11px] font-bold text-accent-foreground shadow-lg', className)}
      title={label}
      aria-label={label}
    >
      {count}
    </span>
  );
}

/** "S1 · E3" or "Special 2" (the same in every Arc app). */
export function episodeCode(season?: number, episode?: number): string {
  if (season === undefined || episode === undefined) return '';
  return season === 0 ? `Special ${episode}` : `S${season} · E${episode}`;
}
