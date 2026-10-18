import type { MouseEvent } from 'react';
import { Link } from 'react-router';
import { Check } from 'lucide-react';
import type { Item } from '@/lib/api';
import { itemHref, posterUrl } from '@/lib/api';
import { useSpoilerGuard } from '@/lib/home';
import { cn } from '@/lib/utils';
import { Artwork } from './Artwork';
import { ContinueCardMenu } from './CardMenu';
import { episodeCode, ProgressBar, progressOf, WatchBadge } from './WatchState';

/** What a card shows, when it's not simply an item's own poster and name. */
export interface CardOverrides {
  image?: string;
  title?: string;
  subtitle?: string;
  /** Leave out the watched tick or unwatched count (it would be the whole show's on a season card). */
  noBadge?: boolean;
}

/**
 * A poster card. An episode shows its show's poster, with the episode number underneath.
 */
export function PosterCard({
  item,
  className,
  overrides,
  onRemove,
  onSelect,
  selected,
}: {
  item: Item;
  className?: string;
  overrides?: CardOverrides;
  /** Offers "Remove from Continue Watching" (on carry-on-watching rows). */
  onRemove?: (item: Item) => void;
  /** When choosing titles: clicking selects instead of opening. */
  onSelect?: (e: MouseEvent) => void;
  selected?: boolean;
}) {
  const isEpisode = item.kind === 'episode';
  const image = overrides?.image ?? (isEpisode ? item.images?.series_primary : posterUrl(item));
  const title = overrides?.title ?? (isEpisode ? (item.series_title ?? item.title) : item.title);
  const subtitle =
    overrides?.subtitle ?? (isEpisode ? [episodeCode(item.season_number, item.episode_number), item.title].filter(Boolean).join(' · ') : item.year || ' ');
  const card = (
    <>
      <div
        className={cn(
          'relative overflow-hidden rounded-xl shadow-lg shadow-black/25 ring-1 ring-border transition-all duration-300 ease-smooth group-hover:-translate-y-1 group-hover:shadow-2xl group-hover:shadow-black/40 group-has-[:focus-visible]:ring-2 group-has-[:focus-visible]:ring-accent',
          selected && 'ring-[3px] ring-accent',
        )}
      >
        <Artwork src={image} alt={title} fallbackTitle={title} icon={item.kind === 'movie' ? 'film' : 'tv'} />
        <div className="pointer-events-none absolute inset-0 bg-gradient-to-t from-black/0 via-transparent to-white/5 opacity-0 transition-opacity duration-300 group-hover:opacity-100" />
        {!overrides?.noBadge && <WatchBadge item={item} />}
        <ProgressBar value={progressOf(item)} />
        {onSelect && (
          <>
            <div className={cn('absolute inset-0 transition-colors', selected ? 'bg-accent/15' : 'group-hover:bg-black/20')} />
            <span
              className={cn(
                'absolute left-2 top-2 flex size-6 items-center justify-center rounded-full ring-2 transition-colors',
                selected ? 'bg-accent text-white ring-accent' : 'bg-black/40 text-transparent ring-white/80',
              )}
            >
              <Check className="size-3.5" strokeWidth={3} />
            </span>
          </>
        )}
      </div>
      <div className="mt-2.5 px-0.5">
        <p className="truncate text-[13px] font-medium leading-tight text-foreground">{title}</p>
        <p className="mt-0.5 truncate text-xs text-subtle">{subtitle}</p>
      </div>
    </>
  );
  return (
    <div className={cn('group relative', className)}>
      {onRemove && <ContinueCardMenu item={item} onRemove={onRemove} />}
      {onSelect ? (
        <button type="button" onClick={onSelect} aria-pressed={!!selected} className="block w-full text-left outline-none">
          {card}
        </button>
      ) : (
        <Link to={itemHref(item)} className="block outline-none" draggable={false}>
          {card}
        </Link>
      )}
    </div>
  );
}

/**
 * A landscape card that opens the title: a movie or show's background, or an episode's still
 * (its show's background while spoilers are hidden).
 */
export function ThumbCard({
  item,
  className,
  overrides,
  onRemove,
}: {
  item: Item;
  className?: string;
  overrides?: CardOverrides;
  /** Offers "Remove from Continue Watching" (on carry-on-watching rows). */
  onRemove?: (item: Item) => void;
}) {
  const isEpisode = item.kind === 'episode';
  const hideSpoilers = useSpoilerGuard()(item);
  const image =
    overrides?.image ??
    (isEpisode ? (hideSpoilers && item.images?.series_backdrop) || item.images?.primary || item.images?.series_backdrop : item.images?.backdrop);
  const title = overrides?.title ?? (isEpisode ? (item.series_title ?? item.title) : item.title);
  const subtitle =
    overrides?.subtitle ?? (isEpisode ? [episodeCode(item.season_number, item.episode_number), item.title].filter(Boolean).join(' · ') : item.year || ' ');
  const blur = hideSpoilers && image === item.images?.primary;
  return (
    <div className={cn('group relative', className)}>
      {onRemove && <ContinueCardMenu item={item} onRemove={onRemove} />}
      <Link to={itemHref(item)} className="block outline-none" draggable={false}>
        <div className="relative overflow-hidden rounded-xl shadow-lg shadow-black/25 ring-1 ring-border transition-all duration-300 ease-smooth group-hover:-translate-y-1 group-hover:shadow-2xl group-has-[:focus-visible]:ring-2 group-has-[:focus-visible]:ring-accent">
          <Artwork src={image} alt="" kind="still" fallbackTitle={title} icon={item.kind === 'movie' ? 'film' : 'tv'} className={cn(blur && 'scale-110 blur-xl')} />
          {item.kind !== 'episode' && item.images?.logo && (
            <img src={item.images.logo} alt="" className="pointer-events-none absolute bottom-3 left-3 max-h-[30%] max-w-[55%] object-contain object-left-bottom drop-shadow-[0_2px_10px_rgb(0_0_0/0.6)]" />
          )}
          {!overrides?.noBadge && <WatchBadge item={item} />}
          <ProgressBar value={progressOf(item)} />
        </div>
        <div className="mt-2.5 px-0.5">
          <p className="truncate text-[13px] font-medium leading-tight text-foreground">{title}</p>
          <p className="mt-0.5 truncate text-xs text-subtle">{subtitle}</p>
        </div>
      </Link>
    </div>
  );
}

export function PosterGrid({
  items,
  select,
}: {
  items: Item[];
  /** Choosing titles: which are chosen, and what a click does (shift-click picks a range). */
  select?: { selected: Set<string>; onToggle: (item: Item, index: number, e: MouseEvent) => void };
}) {
  return (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(132px,1fr))] gap-x-5 gap-y-7 sm:grid-cols-[repeat(auto-fill,minmax(156px,1fr))]">
      {items.map((item, i) => (
        <PosterCard
          key={item.id}
          item={item}
          selected={select?.selected.has(item.id)}
          onSelect={select ? (e) => select.onToggle(item, i, e) : undefined}
        />
      ))}
    </div>
  );
}

export function PosterGridSkeleton({ count = 12 }: { count?: number }) {
  return (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(132px,1fr))] gap-x-5 gap-y-7 sm:grid-cols-[repeat(auto-fill,minmax(156px,1fr))]">
      {Array.from({ length: count }, (_, i) => (
        <div key={i}>
          <div className="aspect-[2/3] animate-pulse rounded-xl bg-surface-2" />
          <div className="mt-2.5 h-3 w-3/4 animate-pulse rounded bg-surface-2" />
        </div>
      ))}
    </div>
  );
}
