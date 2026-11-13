import { useEffect, useRef, useState, type CSSProperties } from 'react';
import { useTranslation } from 'react-i18next';
import { RectangleVertical } from 'lucide-react';
import { usePrefs, useUpdatePrefs } from '@/lib/home';
import { ErrorNotice } from './Page';

type PosterSizeKey = 'home_poster_size' | 'library_poster_size';

/** Preview locally; persist only completed interactions through the shared profile patch. */
export function usePosterSize(key: PosterSizeKey) {
  const prefs = usePrefs();
  const save = useUpdatePrefs();
  const saved = prefs.data?.[key] || 100;
  const [draft, setDraft] = useState<number>();
  const [failed, setFailed] = useState<number>();
  const pending = useRef(false);
  useEffect(() => { setDraft(undefined); }, [saved]);
  const submit = (value: number) => {
    if (pending.current || value === saved) return;
    pending.current = true;
    save.mutate({ [key]: value }, {
      onSuccess: () => { setDraft(undefined); setFailed(undefined); },
      onError: () => { setDraft(undefined); setFailed(value); },
      onSettled: () => { pending.current = false; },
    });
  };
  const percent = draft ?? saved;
  return {
    percent, preview: setDraft, submit,
    style: { '--poster-scale': percent / 100 } as CSSProperties,
    disabled: !prefs.data || save.isPending,
    busy: save.isPending || prefs.isFetching,
    error: save.error || prefs.error,
    retry: () => { if (save.error && failed !== undefined) submit(failed); else void prefs.refetch(); },
  };
}

export function PosterSizeControl({ size }: { size: ReturnType<typeof usePosterSize> }) {
  const { t } = useTranslation('common');
  return (
    <div className="flex max-w-full flex-wrap items-center gap-2">
      <label className="flex items-center gap-2 text-muted" title={t('posterSize.label')}>
        <span className="sr-only">{t('posterSize.label')}</span>
        <RectangleVertical aria-hidden className="size-3 shrink-0" />
        <input type="range" min={75} max={150} step={5} value={size.percent}
          aria-valuetext={t('posterSize.value', { size: size.percent })}
          disabled={size.disabled}
          onChange={event => size.preview(Number(event.target.value))}
          onPointerUp={event => size.submit(Number(event.currentTarget.value))}
          onKeyUp={event => size.submit(Number(event.currentTarget.value))}
          onBlur={event => size.submit(Number(event.currentTarget.value))}
          onPointerCancel={() => size.preview(undefined)}
          className="h-9 w-20 min-w-0 cursor-pointer accent-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent disabled:cursor-wait disabled:opacity-50 sm:w-28" />
        <RectangleVertical aria-hidden className="size-4 shrink-0" />
      </label>
      {size.error && <ErrorNotice error={size.error} onRetry={size.retry} busy={size.busy} />}
    </div>
  );
}
