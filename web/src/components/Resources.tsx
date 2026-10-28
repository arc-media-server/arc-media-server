import { useTranslation } from 'react-i18next';
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api, type ResourceSample } from '@/lib/api';
import * as fmt from '@/lib/format';
import { cn } from '@/lib/utils';

/** The Dashboard's graphs, as Plex's: CPU, memory and data sent, for 5 minutes or a day. */
export function ResourceGraphs() {
  const { t } = useTranslation('reports');
  const [day, setDay] = useState(false);
  const { data } = useQuery({
    queryKey: ['resources', day],
    queryFn: () => api.resources(day),
    refetchInterval: day ? 60_000 : 2_000,
  });
  const samples = data?.samples ?? [];
  const last = samples[samples.length - 1];
  const known = (v: number) => v >= 0;
  const memMax = data && data.mem_total > 0 ? data.mem_total : Math.max(1, ...samples.map((s) => Math.max(s.mem_arc, s.mem_used)));
  const netMax = niceMax(Math.max(1, ...samples.map((s) => s.net_home + s.net_away)));
  return (
    <section>
      <div className="mb-3 flex items-center gap-3">
        <h2 className="mr-auto text-lg font-semibold">{t("reports:resources.server")}</h2>
        <div className="flex rounded-full bg-surface p-0.5 ring-1 ring-border" role="tablist">
          {[
            [false, t('reports:resources.lastMinutes')],
            [true, t('reports:resources.lastDay')],
          ].map(([v, label]) => (
            <button
              key={String(v)}
              role="tab"
              aria-selected={day === v}
              onClick={() => setDay(v as boolean)}
              className={cn('rounded-full px-3 py-1 text-xs font-medium transition-colors', day === v ? 'bg-foreground text-background' : 'text-muted hover:text-foreground')}
            >
              {label}
            </button>
          ))}
        </div>
      </div>
      <div className="grid gap-3 lg:grid-cols-3">
        <Graph
          title={t("reports:resources.cpu")}
          samples={samples}
          max={100}
          series={[
            { label: 'Arc', color: 'var(--color-accent)', value: (s) => s.cpu_arc },
            { label: t('reports:resources.computer'), color: 'var(--color-subtle)', value: (s) => s.cpu_system, hidden: !!last && !known(last.cpu_system) },
          ]}
          format={(v) => `${fmt.number(Math.round(v), { useGrouping: false })}%`}
          note={data ? t('reports:resources.cores', { count: data.cores }) : undefined}
        />
        <Graph
          title={t("reports:resources.memory")}
          samples={samples}
          max={memMax}
          series={[
            { label: 'Arc', color: 'var(--color-accent)', value: (s) => s.mem_arc },
            { label: t('reports:resources.computer'), color: 'var(--color-subtle)', value: (s) => s.mem_used, hidden: !!last && !known(last.mem_used) },
          ]}
          format={(v) => fmt.fileSize(v)}
          note={data && data.mem_total > 0 ? t('reports:resources.totalMemory', { size: fmt.fileSize(data.mem_total) }) : undefined}
        />
        <Graph
          title={t("reports:resources.network")}
          samples={samples}
          max={netMax}
          stacked
          series={[
            { label: t('reports:resources.home'), color: 'var(--color-accent)', value: (s) => s.net_home },
            { label: t('reports:resources.away'), color: '#f59e0b', value: (s) => s.net_away },
          ]}
          format={(v) => bitrate(v)}
          note={t('reports:resources.sentToPlayers')}
        />
      </div>
    </section>
  );
}

interface Series {
  label: string;
  color: string;
  value: (s: ResourceSample) => number;
  hidden?: boolean;
}

/** A small line (or stacked area) chart with its latest values. */
function Graph({ title, samples, max, series, format, note, stacked }: { title: string; samples: ResourceSample[]; max: number; series: Series[]; format: (v: number) => string; note?: string; stacked?: boolean }) {
  const { t } = useTranslation('reports');
  const shown = series.filter((s) => !s.hidden);
  const last = samples[samples.length - 1];
  const W = 300;
  const H = 90;
  const n = samples.length;
  const x = (i: number) => (n <= 1 ? W : (i / (n - 1)) * W);
  const y = (v: number) => H - (Math.max(0, Math.min(v, max)) / max) * H;
  const line = (vals: number[]) => vals.map((v, i) => `${i ? 'L' : 'M'}${x(i).toFixed(1)},${y(v).toFixed(1)}`).join('');
  // Stacked: each series sits on the ones before it.
  const base = samples.map(() => 0);
  return (
    <div className="rounded-2xl bg-surface p-4 ring-1 ring-border">
      <div className="flex items-baseline gap-3">
        <p className="mr-auto text-sm font-semibold">{title}</p>
        {last &&
          shown.map((s) => (
            <span key={s.label} className="flex items-center gap-1.5 text-xs text-muted">
              <span className="size-2 rounded-full" style={{ background: s.color }} />
              {s.label} <span className="font-medium tabular-nums text-foreground">{s.value(last) >= 0 ? format(s.value(last)) : '–'}</span>
            </span>
          ))}
      </div>
      <svg viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" className="mt-3 h-24 w-full overflow-visible" role="img" aria-label={t('reports:resources.overTime', { title })}>
        {[0.25, 0.5, 0.75].map((f) => (
          <line key={f} x1={0} x2={W} y1={H * f} y2={H * f} stroke="var(--color-border)" strokeWidth={1} vectorEffect="non-scaling-stroke" />
        ))}
        {n > 1 &&
          shown
            // A stacked series with nothing in it would only outline the one below.
            .filter((s) => !stacked || samples.some((smp) => s.value(smp) > 0))
            .map((s) => {
              const below = [...base];
              const vals = samples.map((smp, i) => {
                const v = Math.max(0, s.value(smp));
                return stacked ? (base[i] += v) : v;
              });
              const top = line(vals);
              // A stacked series fills the band between the one below and its own top.
              const floor = stacked
                ? below.map((v, i) => `L${x(i).toFixed(1)},${y(v).toFixed(1)}`).reverse().join('')
                : `L${W},${H}L0,${H}`;
              return (
                <g key={s.label}>
                  <path d={`${top}${floor}Z`} fill={s.color} opacity={stacked ? 0.35 : 0.12} />
                  <path d={top} fill="none" stroke={s.color} strokeWidth={1.6} vectorEffect="non-scaling-stroke" />
                </g>
              );
            })}
      </svg>
      <div className="mt-1.5 flex justify-between text-[11px] text-subtle">
        <span>{note}</span>
        <span className="tabular-nums">{format(max)}</span>
      </div>
      {n <= 1 && <p className="-mt-16 mb-10 text-center text-xs text-subtle">{t("reports:resources.gathering")}</p>}
    </div>
  );
}

/** A round top for the network graph, so the scale reads well. */
function niceMax(v: number) {
  const steps = [1, 2, 5];
  let p = 1;
  for (;;) {
    for (const s of steps) if (s * p >= v * 1.15) return s * p;
    p *= 10;
  }
}

/** Bytes a second as bits a second, e.g. "12.4 Mbps". */
function bitrate(bytesPerSec: number) {
  const b = bytesPerSec * 8;
  if (b >= 1e9) return `${fmt.fixed(b / 1e9, 1)} Gbps`;
  if (b >= 1e6) return `${fmt.fixed(b / 1e6, 1)} Mbps`;
  if (b >= 1e3) return `${fmt.number(Math.round(b / 1e3), { useGrouping: false })} kbps`;
  return `${fmt.number(Math.round(b), { useGrouping: false })} bps`;
}
