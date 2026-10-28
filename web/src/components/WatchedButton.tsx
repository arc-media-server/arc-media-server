import { useTranslation } from 'react-i18next';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Check, Circle, Loader2 } from 'lucide-react';
import { api } from '@/lib/api';
import { cn } from '@/lib/utils';
import { ErrorNotice } from './Page';
import { Button, type ButtonProps } from './ui/button';

/** Refetch everything that shows watch state. */
export function useWatchStateChanged() {
  const qc = useQueryClient();
  return () => {
    for (const key of ['item', 'items', 'seasons', 'continue', 'next-up', 'featured']) qc.invalidateQueries({ queryKey: [key] });
  };
}

/**
 * Marks something watched or unwatched for the current profile: a movie or episode, a whole show,
 * or (with `season`) one season of a show.
 */
export function WatchedButton({
  itemId,
  watched,
  season,
  label,
  size = 'lg',
  variant = 'secondary',
  className,
}: {
  itemId: string;
  watched: boolean;
  season?: number;
  /** Show text next to the icon (otherwise just the icon, with a tooltip). */
  label?: boolean;
  size?: ButtonProps['size'];
  variant?: ButtonProps['variant'];
  className?: string;
}) {
  const { t } = useTranslation('titles');
  const changed = useWatchStateChanged();
  const toggle = useMutation({ mutationFn: () => api.setPlayed(itemId, !watched, season), onSuccess: changed });
  const title = season !== undefined
    ? watched ? t('titles:watchedButton.seasonUnwatched') : t('titles:watchedButton.seasonWatched')
    : watched ? t('titles:watchedButton.markUnwatched') : t('titles:watchedButton.markAsWatched');
  const Icon = toggle.isPending ? Loader2 : watched ? Check : Circle;
  return (
    <span className="inline-flex max-w-full flex-wrap items-center gap-2" onClick={e => e.stopPropagation()} onKeyDown={e => e.stopPropagation()}>
    <Button
      variant={variant}
      size={label ? size : 'icon'}
      className={cn(!label && size === 'lg' && 'size-12 rounded-2xl', className)}
      onClick={(e) => {
        e.stopPropagation();
        toggle.mutate();
      }}
      disabled={toggle.isPending}
      title={title}
      aria-label={title}
      aria-pressed={watched}
    >
      <Icon className={cn(toggle.isPending && 'animate-spin', watched && !toggle.isPending && 'text-accent')} strokeWidth={watched ? 3 : 2} />
      {label && (watched ? t("titles:watchedButton.watched") : t("titles:watchedButton.markWatched"))}
    </Button>
    <ErrorNotice error={toggle.error} onRetry={() => toggle.mutate()} />
    </span>
  );
}
