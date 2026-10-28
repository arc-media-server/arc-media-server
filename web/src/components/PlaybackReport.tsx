import { useTranslation } from 'react-i18next';
import { useMemo, useState } from 'react';
import { Link } from 'react-router';
import { useQueries } from '@tanstack/react-query';
import { ChevronRight, MonitorPlay } from 'lucide-react';
import { api, type Compatibility, type Library } from '@/lib/api';
import { kindUnit } from '@/lib/kinds';
import { devicePresets } from '@/lib/playback';
import { cn } from '@/lib/utils';
import { inputClass } from '@/components/ui/field';

/**
 * How each library's titles would play on a device: directly, repackaged (quick, nothing lost) or
 * converted (uses the processor or graphics card), and which titles need converting and why.
 */
export function PlaybackReport({ libraries }: { libraries: Library[] }) {
  const { t } = useTranslation('reports');
  const [device, setDevice] = useState(devicePresets[0].id);
  const preset = devicePresets.find((d) => d.id === device)!;
  const caps = useMemo(() => preset.caps(), [preset]);
  const reports = useQueries({
    queries: libraries.map((lib) => ({
      queryKey: ['compatibility', lib.id, device],
      queryFn: () => api.compatibility(lib.id, caps),
    })),
  });

  return (
    <section aria-label={t("reports:playbackReport.playback")}>
      <div className="mb-1 flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <h2 className="flex items-center gap-2.5 text-xl font-semibold">
          <MonitorPlay className="size-5 text-muted" /> {t("reports:playbackReport.playback")}</h2>
        <select aria-label={t("reports:playbackReport.device")} value={device} onChange={(e) => setDevice(e.target.value)} className={cn(inputClass, 'w-auto')}>
          {devicePresets.map((d) => (
            <option key={d.id} value={d.id}>{d.label}</option>
          ))}
        </select>
      </div>
      <p className="mb-4 max-w-3xl text-sm text-muted">{t("reports:playbackReport.howYourTitlesWould")}</p>
      <div className="divide-y divide-border rounded-2xl bg-surface ring-1 ring-border">
        {libraries.map((lib, i) => (
          <LibraryPlayback key={lib.id} library={lib} report={reports[i]?.data} error={reports[i]?.error} />
        ))}
      </div>
    </section>
  );
}



function LibraryPlayback({ library, report: r, error }: { library: Library; report?: Compatibility; error: Error | null }) {
  const { t } = useTranslation('reports');
  const methods = [
  { key: 'direct', label: t('reports:methods.direct'), bar: 'bg-accent' },
  { key: 'remux', label: t('reports:methods.remux'), bar: 'bg-accent/50' },
  { key: 'transcode', label: t('reports:methods.transcode'), bar: 'bg-warning' },
] as const;
  const [expanded, setExpanded] = useState(false);
  if (error) return <p className="px-4 py-3 text-sm text-warning">{library.name}: {error.message}</p>;
  if (!r) return <div className="m-4 h-10 animate-pulse rounded-lg bg-surface-2" />;
  const reasons = Object.entries(r.reasons).sort((a, b) => b[1] - a[1]);
  const shown = expanded ? r.converted : r.converted.slice(0, 5);
  const unit = library.kind === 'shows' ? 'episode' : kindUnit(library.kind)[0];

  return (
    <div className="px-4 py-3">
      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
        <h3 className="font-medium">{library.name}</h3>
        <p className="text-sm text-muted tabular-nums">
          {methods.map((m) => m.key === 'direct' ? t('reports:playbackReport.directCount', { count: r.counts[m.key] }) : m.key === 'remux' ? t('reports:playbackReport.remuxCount', { count: r.counts[m.key] }) : t('reports:playbackReport.transcodeCount', { count: r.counts[m.key] })).join(' · ')}
          <span className="text-subtle"> {unit === 'episode' ? t('reports:playbackReport.ofEpisodes', { count: r.total }) : unit === 'show' ? t('reports:playbackReport.ofShows', { count: r.total }) : unit === 'video' ? t('reports:playbackReport.ofVideos', { count: r.total }) : t('reports:playbackReport.ofMovies', { count: r.total })}</span>
        </p>
      </div>
      {r.total > 0 && (
        <div className="mt-2 flex h-1.5 overflow-hidden rounded-full bg-surface-2" aria-hidden>
          {methods.map((m) => (
            <div key={m.key} className={m.bar} style={{ width: `${(r.counts[m.key] / r.total) * 100}%` }} />
          ))}
        </div>
      )}
      {r.converted.length > 0 && (
        <details className="mt-2">
          <summary className="cursor-pointer text-sm font-medium text-accent">{t('reports:playbackReport.whyConverted', { count: r.converted.length })}</summary>
          {reasons.length > 0 && <p className="mt-2 text-sm text-muted">{reasons.map(([reason, n]) => `${reason} (${n})`).join(' · ')}</p>}
          <div className="mt-2 divide-y divide-border overflow-hidden rounded-xl ring-1 ring-border">
            {shown.map(({ item, reasons: why }) => (
              <Link
                key={item.id}
                to={item.kind === 'episode' ? `/series/${item.parent_id}` : `/movie/${item.id}`}
                className="flex items-center gap-3 px-4 py-2 transition-colors hover:bg-surface-2"
              >
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium">
                    {item.kind === 'episode' && item.series_title ? `${item.series_title} · ${item.title}` : item.title}
                    {item.year ? <span className="font-normal text-muted"> ({item.year})</span> : null}
                  </p>
                  <p className="truncate text-xs text-subtle">{why.join(' · ')}</p>
                </div>
                <ChevronRight className="size-4 text-subtle" />
              </Link>
            ))}
            {r.converted.length > 5 && (
              <button onClick={() => setExpanded((e) => !e)} className="w-full px-4 py-2 text-left text-sm font-medium text-accent hover:bg-surface-2">
                {expanded ? t("reports:playbackReport.showFewer") : t('reports:playbackReport.showAll', { count: r.converted.length })}
              </button>
            )}
          </div>
        </details>
      )}
    </div>
  );
}
