import { Link } from 'react-router';
import type { Item } from '@/lib/api';
import { itemHref, posterUrl } from '@/lib/api';
import { useSpoilerGuard } from '@/lib/home';
import { cn } from '@/lib/utils';
import { Artwork } from './Artwork';
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
export function PosterCard({ item, className, overrides }: { item: Item; className?: string; overrides?: CardOverrides }) {
  const isEpisode = item.kind === 'episode';
  const image = overrides?.image ?? (isEpisode ? item.images?.series_primary : posterUrl(item));
  const title = overrides?.title ?? (isEpisode ? (item.series_title ?? item.title) : item.title);
  const subtitle =
    overrides?.subtitle ?? (isEpisode ? [episodeCode(item.season_number, item.episode_number), item.title].filter(Boolean).join(' · ') : item.year || ' ');
  return (
    <Link to={itemHref(item)} className={cn('group block outline-none', className)} draggable={false}>
      <div className="relative overflow-hidden rounded-xl shadow-lg shadow-black/25 ring-1 ring-border transition-all duration-300 ease-smooth group-hover:-translate-y-1 group-hover:shadow-2xl group-hover:shadow-black/40 group-focus-visible:ring-2 group-focus-visible:ring-accent">
        <Artwork src={image} alt={title} fallbackTitle={title} icon={item.kind === 'movie' ? 'film' : 'tv'} />
        <div className="pointer-events-none absolute inset-0 bg-gradient-to-t from-black/0 via-transparent to-white/5 opacity-0 transition-opacity duration-300 group-hover:opacity-100" />
        {!overrides?.noBadge && <WatchBadge item={item} />}
        <ProgressBar value={progressOf(item)} />
      </div>
      <div className="mt-2.5 px-0.5">
        <p className="truncate text-[13px] font-medium leading-tight text-foreground">{title}</p>
        <p className="mt-0.5 truncate text-xs text-subtle">{subtitle}</p>
      </div>
    </Link>
  );
}

/**
 * A landscape card that opens the title: a movie or show's background, or an episode's still
 * (its show's background while spoilers are hidden).
 */
export function ThumbCard({ item, className, overrides }: { item: Item; className?: string; overrides?: CardOverrides }) {
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
    <Link to={itemHref(item)} className={cn('group block outline-none', className)} draggable={false}>
      <div className="relative overflow-hidden rounded-xl shadow-lg shadow-black/25 ring-1 ring-border transition-all duration-300 ease-smooth group-hover:-translate-y-1 group-hover:shadow-2xl group-focus-visible:ring-2 group-focus-visible:ring-accent">
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
  );
}

export function PosterGrid({ items }: { items: Item[] }) {
  return (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(132px,1fr))] gap-x-5 gap-y-7 sm:grid-cols-[repeat(auto-fill,minmax(156px,1fr))]">
      {items.map((item) => (
        <PosterCard key={item.id} item={item} />
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
