import { useMemo, useState } from 'react';
import { Link } from 'react-router';
import { useQueries } from '@tanstack/react-query';
import { ChevronRight, MonitorPlay } from 'lucide-react';
import { api, type Compatibility, type Library } from '@/lib/api';
import { kindUnit } from '@/lib/kinds';
import { devicePresets } from '@/lib/playback';
import * as fmt from '@/lib/format';
import { cn } from '@/lib/utils';

/**
 * How each library's titles would play on a device: directly, repackaged (quick, nothing lost) or
 * converted (uses the processor or graphics card), and which titles need converting and why.
 */
export function PlaybackReport({ libraries }: { libraries: Library[] }) {
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
    <section>
      <h2 className="mb-1 flex items-center gap-2.5 text-xl font-semibold">
        <MonitorPlay className="size-5 text-muted" /> Playback
      </h2>
      <p className="mb-4 max-w-3xl text-sm text-muted">
        How your titles play on each kind of device. Repackaging is quick and loses nothing; converting uses the processor or graphics card
        while someone watches.
      </p>
      <div className="mb-5 flex flex-wrap gap-1.5" role="radiogroup" aria-label="Device">
        {devicePresets.map((d) => (
          <button
            key={d.id}
            type="button"
            role="radio"
            aria-checked={d.id === device}
            onClick={() => setDevice(d.id)}
            className={cn(
              'rounded-full px-3.5 py-1.5 text-[13px] font-medium ring-1 transition-colors',
              d.id === device ? 'bg-foreground text-background ring-foreground' : 'bg-surface text-muted ring-border hover:text-foreground',
            )}
          >
            {d.label}
          </button>
        ))}
      </div>
      <div className="space-y-6">
        {libraries.map((lib, i) => (
          <LibraryPlayback key={lib.id} library={lib} report={reports[i]?.data} error={reports[i]?.error} />
        ))}
      </div>
    </section>
  );
}

const methods = [
  { key: 'direct', label: 'Play directly', bar: 'bg-accent' },
  { key: 'remux', label: 'Repackaged', bar: 'bg-accent/50' },
  { key: 'transcode', label: 'Converted', bar: 'bg-warning' },
] as const;

function LibraryPlayback({ library, report: r, error }: { library: Library; report?: Compatibility; error: Error | null }) {
  const [expanded, setExpanded] = useState(false);
  if (error) return <p className="text-sm text-warning">{library.name}: {error.message}</p>;
  if (!r) return <div className="h-28 animate-pulse rounded-2xl bg-surface" />;
  const reasons = Object.entries(r.reasons).sort((a, b) => b[1] - a[1]);
  const shown = expanded ? r.converted : r.converted.slice(0, 5);
  const unit = library.kind === 'shows' ? 'episode' : kindUnit(library.kind)[0];

  return (
    <div className="rounded-2xl bg-surface p-5 ring-1 ring-border">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h3 className="font-semibold">{library.name}</h3>
        <p className="text-sm text-subtle">{fmt.plural(r.total, unit)}</p>
      </div>
      {r.total > 0 && (
        <div className="mt-3 flex h-2 overflow-hidden rounded-full bg-surface-2">
          {methods.map((m) => (
            <div key={m.key} className={m.bar} style={{ width: `${(r.counts[m.key] / r.total) * 100}%` }} />
          ))}
        </div>
      )}
      <div className="mt-3 grid grid-cols-3 gap-3">
        {methods.map((m) => (
          <div key={m.key}>
            <p className="flex items-center gap-1.5 text-xs text-muted">
              <span className={cn('size-2 rounded-full', m.bar)} /> {m.label}
            </p>
            <p className="text-lg font-semibold tabular-nums">{r.counts[m.key]}</p>
          </div>
        ))}
      </div>
      {reasons.length > 0 && (
        <p className="mt-3 text-sm text-muted">
          <span className="text-subtle">Why: </span>
          {reasons.map(([reason, n]) => `${reason} (${n})`).join(' · ')}
        </p>
      )}
      {r.converted.length > 0 && (
        <div className="mt-4 divide-y divide-border overflow-hidden rounded-xl ring-1 ring-border">
          {shown.map(({ item, reasons: why }) => (
            <Link
              key={item.id}
              to={item.kind === 'episode' ? `/series/${item.parent_id}` : `/movie/${item.id}`}
              className="flex items-center gap-3 px-4 py-2.5 transition-colors hover:bg-surface-2"
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
            <button onClick={() => setExpanded((e) => !e)} className="w-full px-4 py-2.5 text-left text-sm font-medium text-accent hover:bg-surface-2">
              {expanded ? 'Show fewer' : `Show all ${r.converted.length} that are converted`}
            </button>
          )}
        </div>
      )}
    </div>
  );
}
