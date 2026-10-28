import { useTranslation } from 'react-i18next';
import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Bookmark, BookmarkCheck, Star, StarHalf } from 'lucide-react';
import { api, type Item } from '@/lib/api';
import { cn } from '@/lib/utils';
import { ErrorNotice } from './Page';
import { Button } from '@/components/ui/button';

/**
 * The profile's own rating: five stars, in halves (stored as 1-10). Clicking the rating it already
 * has clears it.
 */
export function StarRating({ item }: { item: Item }) {
  const { t } = useTranslation('titles');
  const qc = useQueryClient();
  const saved = item.user_data?.rating ?? 0;
  const [hover, setHover] = useState<number | null>(null);
  const [value, setValue] = useState(saved);
  const rate = useMutation({
    mutationFn: (n: number) => api.setRating(item.id, n),
    onMutate: (n) => setValue(n),
    onError: () => setValue(saved),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['item', item.id] }),
  });
  const shown = hover ?? value;
  return (
    <>
    <div
      className="flex items-center gap-0.5"
      role="radiogroup"
      aria-label={t("titles:rating.yourRating")}
      onPointerLeave={() => setHover(null)}
      title={value ? t('titles:rating.current', { value: value / 2 }) : t("titles:rating.rateIt")}
    >
      {[1, 2, 3, 4, 5].map((star) => {
        const full = shown >= star * 2;
        const half = !full && shown === star * 2 - 1;
        return (
          <button
            key={star}
            type="button"
            role="radio"
            aria-checked={value === star * 2}
            aria-label={t('titles:rating.stars', { count: star })}
            className="relative size-7 text-foreground/40 outline-none transition-transform hover:scale-110 focus-visible:ring-2 focus-visible:ring-accent"
            onPointerMove={(e) => {
              const r = e.currentTarget.getBoundingClientRect();
              setHover(e.clientX - r.left < r.width / 2 ? star * 2 - 1 : star * 2);
            }}
            onClick={(e) => {
              const r = e.currentTarget.getBoundingClientRect();
              const n = e.detail === 0 ? star * 2 : e.clientX - r.left < r.width / 2 ? star * 2 - 1 : star * 2;
              rate.mutate(n === value ? 0 : n);
            }}
          >
            <Star className="absolute inset-0 m-auto size-5" />
            {(full || half) && (
              <span className="absolute inset-0 flex items-center justify-center text-amber-400">
                {full ? <Star className="size-5 fill-current" /> : <StarHalf className="size-5 fill-current" />}
              </span>
            )}
          </button>
        );
      })}
    </div>
    <ErrorNotice error={rate.error} onRetry={() => { if (rate.variables !== undefined) rate.mutate(rate.variables); }} />
    </>
  );
}

/** Puts a movie or show on the profile's watchlist, or takes it off. */
export function WatchlistButton({ item }: { item: Item }) {
  const { t } = useTranslation('titles');
  const qc = useQueryClient();
  const [on, setOn] = useState(!!item.user_data?.watchlisted);
  const toggle = useMutation({
    mutationFn: (next: boolean) => api.setWatchlist(item.id, next),
    onMutate: (next) => setOn(next),
    onError: (_e, next) => setOn(!next),
    onSuccess: () => {
      for (const key of ['watchlist', 'item']) qc.invalidateQueries({ queryKey: [key] });
    },
  });
  const label = on ? t('titles:rating.onWatchlist') : t('titles:rating.addWatchlist');
  return (
    <>
    <Button
      variant="secondary"
      size="lg"
      className={cn('size-12 rounded-2xl px-0', on && 'text-accent')}
      onClick={() => toggle.mutate(!on)}
      title={label}
      aria-label={label}
      aria-pressed={on}
    >
      {on ? <BookmarkCheck className="fill-current" /> : <Bookmark />}
    </Button>
    <ErrorNotice error={toggle.error} onRetry={() => { if (toggle.variables !== undefined) toggle.mutate(toggle.variables); }} />
    </>
  );
}
