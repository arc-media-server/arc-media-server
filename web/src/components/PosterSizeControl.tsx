import { createContext, useContext, useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { LayoutGrid, Square } from 'lucide-react';
import { usePrefs, useUpdatePrefs } from '@/lib/home';
import { ErrorNotice } from './Page';

/**
 * The profile's one card size, for every page with media cards (Home, libraries, collections,
 * search, watchlist, playlists). The server keeps two values from when Home and libraries were
 * sized apart; both are saved together and read as one (Home's first).
 */
function useCardSizeState() {
  const prefs = usePrefs();
  const save = useUpdatePrefs();
  const saved = prefs.data?.home_poster_size || prefs.data?.library_poster_size || 100;
  const [draft, setDraft] = useState<number>();
  const [failed, setFailed] = useState<number>();
  const pending = useRef(false);
  useEffect(() => { setDraft(undefined); }, [saved]);
  // Dragging previews at once; only the finished change is saved, once.
  const submit = (value: number) => {
    if (pending.current || value === saved) {
      if (value === saved) setDraft(undefined);
      return;
    }
    pending.current = true;
    save.mutate({ home_poster_size: value, library_poster_size: value }, {
      onSuccess: () => { setDraft(undefined); setFailed(undefined); },
      onError: () => { setDraft(undefined); setFailed(value); },
      onSettled: () => { pending.current = false; },
    });
  };
  const percent = draft ?? saved;
  return {
    percent, preview: setDraft, submit,
    disabled: !prefs.data || save.isPending,
    busy: save.isPending || prefs.isFetching,
    error: save.error || prefs.error,
    retry: () => { if (save.error && failed !== undefined) submit(failed); else void prefs.refetch(); },
  };
}

type CardSize = ReturnType<typeof useCardSizeState>;
const CardSizeContext = createContext<CardSize | null>(null);

/** Gives the pages inside it the profile's card size (the `--card-scale` the card styles use). */
export function CardSizeProvider({ children }: { children: (style: CSSProperties) => ReactNode }) {
  const size = useCardSizeState();
  return <CardSizeContext.Provider value={size}>{children({ '--card-scale': size.percent / 100 } as CSSProperties)}</CardSizeContext.Provider>;
}

/** The card size slider: smaller cards at the left, larger at the right. */
export function PosterSizeControl() {
  const { t } = useTranslation('common');
  const size = useContext(CardSizeContext);
  if (!size) return null;
  return (
    <div className="flex max-w-full flex-wrap items-center gap-2">
      <label className="flex items-center gap-2.5 text-muted" title={t('posterSize.label')}>
        <span className="sr-only">{t('posterSize.label')}</span>
        <LayoutGrid aria-hidden className="size-3.5 shrink-0" />
        <input type="range" min={75} max={150} step={5} value={size.percent}
          aria-valuetext={t('posterSize.value', { size: size.percent })}
          disabled={size.disabled}
          onChange={event => size.preview(Number(event.target.value))}
          onPointerUp={event => size.submit(Number(event.currentTarget.value))}
          onKeyUp={event => size.submit(Number(event.currentTarget.value))}
          onBlur={event => size.submit(Number(event.currentTarget.value))}
          onPointerCancel={() => size.preview(undefined)}
          style={{ '--fill': `${((size.percent - 75) / 75) * 100}%` } as CSSProperties}
          className="card-size-range w-24 min-w-0 sm:w-28" />
        <Square aria-hidden className="size-4 shrink-0" />
      </label>
      {size.error && <ErrorNotice error={size.error} onRetry={size.retry} busy={size.busy} />}
    </div>
  );
}
