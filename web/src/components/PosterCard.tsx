import { useState, type MouseEvent } from 'react';
import { Link } from 'react-router';
import { Check } from 'lucide-react';
import { CardActions, type CardHover } from './CardActions';
import type { Item } from '@/lib/api';
import { itemHref, posterUrl, type Collection } from '@/lib/api';
import { CollectionCard } from '@/components/Collections';
import { useSpoilerGuard } from '@/lib/home';
import { cn } from '@/lib/utils';
import { cardYear } from '@/lib/kinds';
import { ArcArtwork } from './ArcArtwork';
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
  hover,
}: {
  item: Item;
  className?: string;
  overrides?: CardOverrides;
  /** Offers "Remove from Continue Watching" (on carry-on-watching rows). */
  onRemove?: (item: Item) => void;
  /** When choosing titles: clicking selects instead of opening. */
  onSelect?: (e: MouseEvent) => void;
  selected?: boolean;
  /** Actions shown on hover (Play, ⋯, and for the server owner select and edit). */
  hover?: CardHover;
}) {
  const isEpisode = item.kind === 'episode';
  const [menuOpen, setMenuOpen] = useState(false);
  const image = overrides?.image ?? (isEpisode ? item.images?.series_primary : posterUrl(item));
  const title = overrides?.title ?? (isEpisode ? (item.series_title ?? item.title) : item.title);
  const subtitle =
    overrides?.subtitle ?? (isEpisode ? [episodeCode(item.season_number, item.episode_number), item.title].filter(Boolean).join(' · ') : cardYear(item));
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
        {onSelect && <SelectMark selected={!!selected} />}
      </div>
      <div className="mt-2.5 px-0.5">
        <p className="truncate text-[13px] font-medium leading-tight text-foreground">{title}</p>
        <p className="mt-0.5 truncate text-xs text-subtle">{subtitle}</p>
      </div>
    </>
  );
  return (
    <div className={cn('group relative scroll-mt-4', className)} data-item={item.id}
      onContextMenu={hover && !onSelect ? (e) => { e.preventDefault(); setMenuOpen(true); } : undefined}>
      {onRemove && <ContinueCardMenu item={item} onRemove={onRemove} />}
      {hover && !onSelect && <CardActions item={item} title={title} hover={hover} landscape={false} menuOpen={menuOpen} onMenuOpen={setMenuOpen} />}
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
  onSelect,
  selected,
  hover,
  arc,
}: {
  item: Item;
  className?: string;
  overrides?: CardOverrides;
  /** An Arc thumbnail (see ArcArtwork) instead of a landscape one. */
  arc?: boolean;
  /** Offers "Remove from Continue Watching" (on carry-on-watching rows). */
  onRemove?: (item: Item) => void;
  /** When choosing titles: clicking selects instead of opening. */
  onSelect?: (e: MouseEvent) => void;
  selected?: boolean;
  /** Actions shown on hover (Play, ⋯, and for the server owner select and edit). */
  hover?: CardHover;
}) {
  const isEpisode = item.kind === 'episode';
  const [menuOpen, setMenuOpen] = useState(false);
  const hideSpoilers = useSpoilerGuard()(item);
  const image =
    overrides?.image ??
    (isEpisode ? (hideSpoilers && item.images?.series_backdrop) || item.images?.primary || item.images?.series_backdrop : item.images?.backdrop);
  const title = overrides?.title ?? (isEpisode ? (item.series_title ?? item.title) : item.title);
  const subtitle =
    overrides?.subtitle ?? (isEpisode ? [episodeCode(item.season_number, item.episode_number), item.title].filter(Boolean).join(' · ') : cardYear(item));
  const blur = hideSpoilers && image === item.images?.primary;
  const card = (
    <>
        <div
          className={cn(
            'relative overflow-hidden rounded-xl shadow-lg shadow-black/25 ring-1 ring-border transition-all duration-300 ease-smooth group-hover:-translate-y-1 group-hover:shadow-2xl group-has-[:focus-visible]:ring-2 group-has-[:focus-visible]:ring-accent',
            selected && 'ring-[3px] ring-accent',
          )}
        >
          {arc ? (
            <ArcArtwork item={item} title={title} image={overrides?.image} />
          ) : (
            <Artwork src={image} alt="" kind="still" fallbackTitle={title} icon={item.kind === 'movie' ? 'film' : 'tv'} className={cn(blur && 'scale-110 blur-xl')} />
          )}
          {!arc && item.kind !== 'episode' && item.images?.logo && (
            <img src={item.images.logo} alt="" className="pointer-events-none absolute bottom-3 left-3 max-h-[30%] max-w-[55%] object-contain object-left-bottom drop-shadow-[0_2px_10px_rgb(0_0_0/0.6)]" />
          )}
          {!overrides?.noBadge && <WatchBadge item={item} />}
          <ProgressBar value={progressOf(item)} />
          {onSelect && <SelectMark selected={!!selected} />}
        </div>
        <div className="mt-2.5 px-0.5">
          <p className="truncate text-[13px] font-medium leading-tight text-foreground">{title}</p>
          <p className="mt-0.5 truncate text-xs text-subtle">{subtitle}</p>
        </div>
    </>
  );
  return (
    <div className={cn('group relative scroll-mt-4', className)} data-item={item.id}
      onContextMenu={hover && !onSelect ? (e) => { e.preventDefault(); setMenuOpen(true); } : undefined}>
      {onRemove && <ContinueCardMenu item={item} onRemove={onRemove} />}
      {hover && !onSelect && <CardActions item={item} title={title} hover={hover} landscape={true} menuOpen={menuOpen} onMenuOpen={setMenuOpen} />}
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

/** The tick on a card while choosing titles. */
function SelectMark({ selected }: { selected: boolean }) {
  return (
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
  );
}

export function PosterGrid({
  items,
  select,
  landscape,
  collections,
  collectionUnit = 'title',
  hover,
}: {
  items: Item[];
  /** Collections shown among the titles (Plex's collection mode): each takes the place of its first
   * title, and hides its titles when it says so. */
  collections?: { collection: Collection; hideItems: boolean }[];
  collectionUnit?: string;
  /** Choosing titles: which are chosen, and what a click does (shift-click picks a range). */
  select?: { selected: Set<string>; onToggle: (item: Item, index: number, e: MouseEvent) => void };
  /** Landscape thumbnails instead of posters (home videos). */
  landscape?: boolean;
  /** Hover actions on each card, while not choosing titles (select and edit for the server owner). */
  hover?: { onSelect?: (item: Item, index: number, e: MouseEvent) => void; onEdit?: (item: Item) => void };
}) {
  const Card = landscape ? ThumbCard : PosterCard;
  const hoverFor = (item: Item, i: number): CardHover | undefined =>
    hover && !select ? { onSelect: hover.onSelect && ((e) => hover.onSelect!(item, i, e)), onEdit: hover.onEdit && (() => hover.onEdit!(item)) } : undefined;
  if (collections?.length) {
    // Where each collection goes: at its first title; and the titles it hides.
    const hidden = new Set<string>();
    const at = new Map<string, Collection[]>();
    for (const { collection, hideItems } of collections) {
      const ids = new Set(collection.item_ids);
      const first = items.find((it) => ids.has(it.id));
      if (!first) continue;
      at.set(first.id, [...(at.get(first.id) ?? []), collection]);
      if (hideItems) for (const id of ids) hidden.add(id);
    }
    return (
      <div className="grid grid-cols-[repeat(auto-fill,minmax(132px,1fr))] gap-x-5 gap-y-7 sm:grid-cols-[repeat(auto-fill,minmax(156px,1fr))]">
        {items.flatMap((item, i) => [
          ...(at.get(item.id) ?? []).map((c) => <CollectionCard key={`c-${c.id}`} collection={c} unit={collectionUnit} />),
          ...(hidden.has(item.id) ? [] : [<PosterCard key={item.id} item={item} hover={hoverFor(item, i)} />]),
        ])}
      </div>
    );
  }
  return (
    <div
      className={cn(
        'grid gap-x-5 gap-y-7',
        landscape
          ? 'grid-cols-[repeat(auto-fill,minmax(220px,1fr))] sm:grid-cols-[repeat(auto-fill,minmax(260px,1fr))]'
          : 'grid-cols-[repeat(auto-fill,minmax(132px,1fr))] sm:grid-cols-[repeat(auto-fill,minmax(156px,1fr))]',
      )}
    >
      {items.map((item, i) => (
        <Card
          key={item.id}
          item={item}
          selected={select?.selected.has(item.id)}
          onSelect={select ? (e) => select.onToggle(item, i, e) : undefined}
          hover={hoverFor(item, i)}
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
