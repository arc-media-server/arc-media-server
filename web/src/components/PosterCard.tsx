import { Link } from 'react-router';
import type { Item } from '@/lib/api';
import { itemHref, posterUrl } from '@/lib/api';
import { cn } from '@/lib/utils';
import { Artwork } from './Artwork';
import { ProgressBar, progressOf, WatchBadge } from './WatchState';

export function PosterCard({ item, className }: { item: Item; className?: string }) {
  return (
    <Link to={itemHref(item)} className={cn('group block outline-none', className)} draggable={false}>
      <div className="relative overflow-hidden rounded-xl shadow-lg shadow-black/25 ring-1 ring-border transition-all duration-300 ease-smooth group-hover:-translate-y-1 group-hover:shadow-2xl group-hover:shadow-black/40 group-focus-visible:ring-2 group-focus-visible:ring-accent">
        <Artwork
          src={posterUrl(item)}
          alt={item.title}
          fallbackTitle={item.title}
          icon={item.kind === 'series' ? 'tv' : 'film'}
        />
        <div className="pointer-events-none absolute inset-0 bg-gradient-to-t from-black/0 via-transparent to-white/5 opacity-0 transition-opacity duration-300 group-hover:opacity-100" />
        <WatchBadge item={item} />
        <ProgressBar value={progressOf(item)} />
      </div>
      <div className="mt-2.5 px-0.5">
        <p className="truncate text-[13px] font-medium leading-tight text-foreground">{item.title}</p>
        <p className="mt-0.5 text-xs text-subtle">{item.year || ' '}</p>
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
