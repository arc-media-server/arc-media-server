import { useTranslation } from 'react-i18next';
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Check, Download, Loader2, X } from 'lucide-react';
import { api, type OnlineSubtitle, type SubtitleTrack } from '@/lib/api';
import * as fmt from '@/lib/format';
import { cn } from '@/lib/utils';

// Languages people most often look for, as 3-letter codes.
const languages = ['eng', 'fra', 'deu', 'spa', 'ita', 'por', 'nld', 'swe', 'nor', 'dan', 'fin', 'pol', 'ces', 'hun', 'ron', 'ell', 'tur', 'rus', 'ukr', 'ara', 'heb', 'hin', 'jpn', 'kor', 'zho', 'tha', 'vie', 'ind'];

/** A panel in the player for finding subtitles on OpenSubtitles and adding one. */
export function SubtitleSearch({
  itemId,
  fileId,
  defaultLanguage,
  onClose,
  onAdded,
}: {
  itemId: string;
  fileId: number;
  defaultLanguage: string;
  onClose: () => void;
  onAdded: (track: SubtitleTrack) => void;
}) {
  const { t } = useTranslation('player');
  const [language, setLanguage] = useState(languages.includes(defaultLanguage) ? defaultLanguage : 'eng');
  const [busy, setBusy] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const results = useQuery({
    queryKey: ['subtitle-search', itemId, fileId, language],
    queryFn: () => api.searchSubtitles(itemId, fileId, language),
    staleTime: 5 * 60_000,
    retry: false,
  });

  const add = async (s: OnlineSubtitle) => {
    setBusy(s.file_id);
    setError(null);
    try {
      const { track } = await api.downloadSubtitle(itemId, fileId, s);
      onAdded(track);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(null);
    }
  };

  return (
    <div
      className="absolute inset-y-0 right-0 z-10 flex w-[min(420px,100%)] flex-col bg-black/85 text-white backdrop-blur-xl"
      onClick={(e) => e.stopPropagation()}
      onMouseMove={(e) => e.stopPropagation()}
    >
      <div className="flex items-center justify-between gap-3 border-b border-white/10 p-5">
        <div>
          <h2 className="text-lg font-semibold">{t("player:subtitleSearch.findSubtitles")}</h2>
          <p className="text-xs text-white/50">{t("player:subtitleSearch.fromOpensubtitlesCom")}</p>
        </div>
        <button type="button" aria-label={t("player:subtitleSearch.close")} onClick={onClose} className="rounded-full p-2 text-white/70 hover:bg-white/10 hover:text-white">
          <X className="size-5" />
        </button>
      </div>
      <div className="p-5 pb-3">
        <select
          value={language}
          onChange={(e) => setLanguage(e.target.value)}
          aria-label={t("player:subtitleSearch.language")}
          className="h-10 w-full rounded-xl bg-white/10 px-3 text-sm text-white outline-none ring-1 ring-white/15 focus:ring-2 focus:ring-white/40 [&>option]:bg-neutral-900"
        >
          {languages.map((l) => (
            <option key={l} value={l}>
              {fmt.language(l)}
            </option>
          ))}
        </select>
      </div>
      {error && <p className="mx-5 mb-2 rounded-lg bg-warning/15 px-3 py-2 text-sm text-warning">{error}</p>}
      <div className="min-h-0 flex-1 overflow-y-auto px-3 pb-5">
        {results.isLoading && (
          <div className="flex justify-center py-10">
            <Loader2 className="size-6 animate-spin text-white/60" />
          </div>
        )}
        {results.error && <p className="px-2 py-6 text-sm text-white/70">{results.error.message}</p>}
        {results.data?.length === 0 && <p className="px-2 py-6 text-sm text-white/60">{t('player:subtitleSearch.noneFound', { language: fmt.language(language) })}</p>}
        {results.data?.map((s) => (
          <button
            key={s.file_id}
            type="button"
            disabled={busy !== null}
            onClick={() => add(s)}
            className="group flex w-full items-start gap-3 rounded-xl px-3 py-2.5 text-left transition-colors hover:bg-white/10 disabled:opacity-60"
          >
            <div className="min-w-0 flex-1">
              <p className="break-words text-sm">{s.release || s.file_name}</p>
              <p className="mt-0.5 flex flex-wrap gap-x-2 text-xs text-white/50">
                {s.hash_match && (
                  <span className="inline-flex items-center gap-1 text-accent">
                    <Check className="size-3" /> {t("player:subtitleSearch.madeForThisFile")}</span>
                )}
                {s.hearing_impaired && <span>{t("player:subtitleSearch.sdh")}</span>}
                {s.forced && <span>{t("player:subtitleSearch.forced")}</span>}
                {s.machine_translated && <span className="text-warning">{t("player:subtitleSearch.machineTranslated")}</span>}
                <span>{t('player:subtitleSearch.downloads', { count: s.downloads })}</span>
              </p>
            </div>
            <span className={cn('mt-0.5 shrink-0 text-white/50 group-hover:text-white')}>
              {busy === s.file_id ? <Loader2 className="size-4 animate-spin" /> : <Download className="size-4" />}
            </span>
          </button>
        ))}
      </div>
    </div>
  );
}
