import type { ComponentProps } from 'react';
import type { Item } from '@/lib/api';
import { ResumeCard } from './EpisodeCard';
import { PosterCard, ThumbCard } from './PosterCard';
import { RowShell } from './RowShell';
export { RowShell } from './RowShell';

export type CardStyle = 'poster' | 'thumb' | 'arc';

/** The width of a card in a row. */
export function cardClass(style: CardStyle) {
  if (style === 'arc') return 'w-[208px] shrink-0 snap-start sm:w-[240px]';
  return style === 'thumb' ? 'w-[260px] shrink-0 snap-start sm:w-[300px]' : 'poster-row-card shrink-0 snap-start';
}

/**
 * A titled, horizontally scrolling row of posters, landscape thumbnails or backdrop cards, with visible controls whenever
 * the content overflows. With `play`, thumbnails play what they show (Continue Watching, Up Next) instead of
 * opening it.
 */
export function MediaRow({
  title,
  items,
  href,
  style = 'poster',
  play,
  seasonPosters,
  onRemove,
  loading,
  error,
  onRetry,
  cardProps,
}: {
  title: string;
  items: Item[];
  href?: string;
  style?: CardStyle;
  play?: boolean;
  /** Episodes on a poster row show their season's poster (when it has one) instead of the show's. */
  seasonPosters?: boolean;
  /** Cards offer "Remove from Continue Watching". */
  onRemove?: (item: Item) => void;
  loading?: boolean;
  error?: unknown;
  onRetry?: () => void;
  cardProps?: (item: Item) => Pick<ComponentProps<typeof PosterCard>, 'hover' | 'onSelect' | 'selected'>;
}) {
  return (
    <RowShell title={title} href={href} empty={items.length === 0} loading={loading} error={error} onRetry={onRetry}>
      {items.map((item) =>
        style === 'poster' ? (
          <PosterCard
            key={item.id}
            {...cardProps?.(item)}
            item={item}
            className={cardClass(style)}
            overrides={seasonPosters && item.images?.season_primary ? { image: item.images.season_primary } : undefined}
            onRemove={onRemove}
          />
        ) : play ? (
          <ResumeCard {...cardProps?.(item)} key={item.id} item={item} className={cardClass(style)} arc={style === 'arc'} onRemove={onRemove} />
        ) : (
          <ThumbCard {...cardProps?.(item)} key={item.id} item={item} className={cardClass(style)} arc={style === 'arc'} onRemove={onRemove} />
        ),
      )}
    </RowShell>
  );
}
