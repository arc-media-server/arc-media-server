import { useState } from 'react';
import { Film, Tv } from 'lucide-react';
import { cn } from '@/lib/utils';

interface ArtworkProps {
  src?: string;
  alt: string;
  kind?: 'poster' | 'still' | 'backdrop';
  className?: string;
  /** Shown on the placeholder when there's no image. */
  fallbackTitle?: string;
  icon?: 'film' | 'tv';
  /** Load immediately instead of when scrolled into view (for above-the-fold artwork). */
  eager?: boolean;
}

/**
 * An image that fades in when loaded and falls back to a tasteful placeholder (gradient,
 * icon and title) when missing or broken, so grids never show broken-image icons.
 */
export function Artwork({ src, alt, kind = 'poster', className, fallbackTitle, icon = 'film', eager }: ArtworkProps) {
  const [loaded, setLoaded] = useState(false);
  const [failed, setFailed] = useState(false);
  const showImage = src && !failed;
  const Icon = icon === 'tv' ? Tv : Film;

  return (
    <div
      className={cn(
        'relative overflow-hidden bg-surface-2',
        kind === 'poster' && 'aspect-[2/3]',
        kind === 'still' && 'aspect-video',
        className,
      )}
    >
      {!showImage || !loaded ? (
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 bg-gradient-to-br from-surface-2 to-surface-3 p-3 text-center">
          {(!showImage || failed) && (
            <>
              <Icon className="size-6 text-subtle" strokeWidth={1.5} />
              {fallbackTitle && kind !== 'backdrop' && (
                <span className="line-clamp-3 text-xs font-medium text-muted">{fallbackTitle}</span>
              )}
            </>
          )}
        </div>
      ) : null}
      {showImage && (
        <img
          src={src}
          alt={alt}
          loading={eager ? 'eager' : 'lazy'}
          decoding="async"
          draggable={false}
          onLoad={() => setLoaded(true)}
          onError={() => setFailed(true)}
          className={cn(
            'absolute inset-0 size-full object-cover transition-opacity duration-500 ease-smooth',
            loaded ? 'opacity-100' : 'opacity-0',
          )}
        />
      )}
    </div>
  );
}
