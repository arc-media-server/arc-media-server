import i18n from '@/i18n';
import { useTranslation } from 'react-i18next';
import { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, Copy, Loader2, MonitorPlay, Pause, Play, Square } from 'lucide-react';
import { api, type LogLine, type Playback } from '@/lib/api';
import * as fmt from '@/lib/format';
import { cn } from '@/lib/utils';
import { Artwork } from '@/components/Artwork';
import { Avatar } from '@/components/Avatar';
import { EmptyState, ErrorState, Page, PageHeader } from '@/components/Page';
import { ActivityList } from '@/components/Activity';
import { ResourceGraphs } from '@/components/Resources';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { inputClass } from '@/components/ui/field';

export function DashboardPage() {
  const { t } = useTranslation('reports');
  const playing = useQuery({ queryKey: ['playing'], queryFn: api.playing, refetchInterval: 3000 });
  const [stopping, setStopping] = useState<Playback | null>(null);

  if (playing.error) return <ErrorState error={playing.error} />;

  return (
    <Page className="pb-16">
      <PageHeader title={t("reports:dashboard.dashboard")} subtitle={t("reports:dashboard.whatSBeingWatched")} />
      <div className="space-y-10 px-6 md:px-10">
        <section>
          <h2 className="mb-3 text-lg font-semibold">{t("reports:dashboard.nowPlaying")}{playing.data && playing.data.length > 0 && <span className="ml-2 text-muted">{playing.data.length}</span>}
          </h2>
          {!playing.data && <div className="h-36 animate-pulse rounded-2xl bg-surface" />}
          {playing.data?.length === 0 && (
            <EmptyState icon={<MonitorPlay />} title={t("reports:dashboard.nothingIsPlaying")}>{t("reports:dashboard.whenSomeoneWatchesSomething")}</EmptyState>
          )}
          <div className="grid gap-3 lg:grid-cols-2">
            {playing.data?.map((p) => <PlaybackCard key={p.id} playback={p} onStop={() => setStopping(p)} />)}
          </div>
        </section>
        <ResourceGraphs />
        <section>
          <h2 className="mb-3 text-lg font-semibold">{t("reports:dashboard.activity")}</h2>
          <div className="rounded-2xl bg-surface p-5 ring-1 ring-border">
            <ActivityList recent={10} />
          </div>
        </section>
        <ServerLog />
      </div>
      <StopDialog playback={stopping} onClose={() => setStopping(null)} />
    </Page>
  );
}

function playbackTitle(p: Playback) {
  const it = p.item;
  if (it.kind === 'episode' && it.series_title) {
    const code = it.season_number != null && it.episode_number != null ? `S${it.season_number} E${it.episode_number} · ` : '';
    return { title: it.series_title, subtitle: `${code}${it.title}` };
  }
  return { title: it.title, subtitle: it.year ? String(it.year) : undefined };
}

function PlaybackCard({ playback: p, onStop }: { playback: Playback; onStop: () => void }) {
  const { t } = useTranslation('reports');
  const { title, subtitle } = playbackTitle(p);
  const href = p.item.kind === 'episode' ? `/series/${p.item.parent_id}` : `/movie/${p.item.id}`;
  const image = p.item.images?.backdrop ?? p.item.images?.series_backdrop ?? p.item.images?.primary;
  const pct = p.duration_ms > 0 ? Math.min(100, (p.position_ms / p.duration_ms) * 100) : 0;
  const StateIcon = p.state === 'paused' ? Pause : p.state === 'buffering' ? Loader2 : Play;

  return (
    <div className="flex overflow-hidden rounded-2xl bg-surface ring-1 ring-border">
      <Link to={href} className="relative hidden w-48 shrink-0 sm:block">
        <Artwork src={image} alt="" kind="backdrop" className="absolute inset-0 size-full" fallbackTitle={title} />
      </Link>
      <div className="flex min-w-0 flex-1 flex-col gap-3 p-4">
        <div className="flex items-start gap-3">
          <div className="min-w-0 flex-1">
            <Link to={href} className="block truncate font-semibold hover:underline">
              {title}
            </Link>
            <p className="truncate text-sm text-muted">{p.extra ? t('reports:dashboard.extraTitle', { title: p.extra }) : subtitle}</p>
          </div>
          <Button variant="secondary" size="sm" onClick={onStop}>
            <Square className="fill-current" /> {t("reports:dashboard.stop")}</Button>
        </div>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-[13px] text-muted">
          <span className="flex items-center gap-1.5 text-foreground">
            <Avatar name={p.profile.name || '?'} color={p.profile.color} className="size-5 text-[10px]" />
            {p.profile.name || t("reports:dashboard.unknownProfile")}
          </span>
          <span>{p.device}</span>
          <span className="text-subtle">{p.address}</span>
          <span className="rounded-full bg-surface-2 px-2 py-0.5 text-[11px] font-medium text-muted ring-1 ring-border">
            {p.method === 'direct' ? t("reports:dashboard.directPlay") : p.throttled ? t("reports:dashboard.convertingThrottled") : t("reports:dashboard.converting")}
          </span>
        </div>
        <div className="mt-auto flex items-center gap-3 text-[12px] tabular-nums text-muted">
          <StateIcon className={cn('size-3.5 shrink-0 fill-current', p.state === 'buffering' && 'animate-spin fill-none')} aria-label={p.state} />
          <span>{fmt.clock(p.position_ms)}</span>
          <div className="h-1 flex-1 overflow-hidden rounded-full bg-surface-2">
            <div className="h-full rounded-full bg-accent transition-[width] duration-1000" style={{ width: `${pct}%` }} />
          </div>
          <span>{p.duration_ms ? fmt.clock(p.duration_ms) : '–'}</span>
        </div>
      </div>
    </div>
  );
}

function StopDialog({ playback, onClose }: { playback: Playback | null; onClose: () => void }) {
  const { t } = useTranslation('reports');
  const qc = useQueryClient();
  const [message, setMessage] = useState('');
  const stop = useMutation({
    mutationFn: (p: Playback) => api.stopPlaying(p.id, message),
    onSuccess: () => {
      onClose();
      setMessage('');
      return qc.invalidateQueries({ queryKey: ['playing'] });
    },
  });
  const name = playback?.profile.name || t('reports:dashboard.thisProfile');

  return (
    <ConfirmDialog
      open={playback !== null}
      onOpenChange={(o) => {
        if (!o) {
          onClose();
          stop.reset();
        }
      }}
      title={playback ? t('reports:dashboard.stopTitle', { title: playbackTitle(playback).title }) : ''}
      confirmLabel={t("reports:dashboard.stop")}
      destructive
      busy={stop.isPending}
      error={stop.error?.message}
      onConfirm={() => playback && stop.mutate(playback)}
    >
      <p>{t('reports:dashboard.stopDescription', { name, device: playback?.device ?? '' })}</p>
      <input
        value={message}
        onChange={(e) => setMessage(e.target.value)}
        maxLength={300}
        placeholder={t("reports:dashboard.messageToShowThem")}
        className={inputClass}
      />
    </ConfirmDialog>
  );
}

const levels = [
  { value: 'debug', get label() { return i18n.t('reports:logLevels.allLevels'); } },
  { value: 'info', get label() { return i18n.t('reports:logLevels.info'); } },
  { value: 'warn', get label() { return i18n.t('reports:logLevels.warnings'); } },
  { value: 'error', get label() { return i18n.t('reports:logLevels.errors'); } },
] as const;

const levelStyle: Record<string, string> = {
  DEBUG: 'text-subtle',
  INFO: 'text-muted',
  WARN: 'text-warning',
  ERROR: 'text-red-400',
};

// How many lines the page keeps.
const maxLines = 1000;

function ServerLog() {
  const { t } = useTranslation('reports');
  const [level, setLevel] = useState<(typeof levels)[number]['value']>('info');
  const [lines, setLines] = useState<LogLine[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  // Fetch the log, then only what's new every few seconds.
  useEffect(() => {
    let cancelled = false;
    let after = 0;
    setLines([]);
    const load = async () => {
      try {
        const got = await api.logs(level, after);
        if (cancelled) return;
        setError(null);
        if (got.length) {
          after = got[got.length - 1].seq;
          setLines((prev) => [...prev, ...got].slice(-maxLines));
        }
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e));
      }
    };
    load();
    const timer = window.setInterval(load, 4000);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [level]);

  const newestFirst = useMemo(() => [...lines].reverse(), [lines]);

  const copy = async () => {
    const text = lines.map((l) => `${l.time} ${l.level} ${l.message}${l.attrs ? ` ${l.attrs}` : ''}`).join('\n');
    await navigator.clipboard.writeText(text);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 2000);
  };

  return (
    <section>
      <div className="mb-3 flex flex-wrap items-center gap-3">
        <h2 className="mr-auto text-lg font-semibold">{t("reports:dashboard.serverLog")}</h2>
        <div className="flex gap-1">
          {levels.map((l) => (
            <button
              key={l.value}
              type="button"
              onClick={() => setLevel(l.value)}
              className={cn(
                'rounded-full px-3.5 py-1 text-[13px] font-medium transition-colors',
                level === l.value ? 'bg-foreground text-background' : 'text-muted hover:text-foreground',
              )}
            >
              {l.label}
            </button>
          ))}
        </div>
        <Button variant="secondary" size="sm" onClick={copy} disabled={lines.length === 0}>
          {copied ? <Check /> : <Copy />} {copied ? t("reports:dashboard.copied") : t("reports:dashboard.copy")}
        </Button>
      </div>
      {error && <p className="mb-2 text-sm text-warning">{t('reports:dashboard.logError', { error })}</p>}
      <div className="max-h-[480px] overflow-auto rounded-2xl bg-surface p-2 font-mono text-[12px] leading-relaxed ring-1 ring-border">
        {newestFirst.length === 0 && <p className="p-3 font-sans text-sm text-muted">{t("reports:dashboard.nothingLoggedAtThis")}</p>}
        {newestFirst.map((l) => (
          <div key={l.seq} className="flex gap-3 rounded-lg px-2 py-0.5 hover:bg-surface-2">
            <span className="shrink-0 text-subtle tabular-nums" title={fmt.dateTime(l.time)}>
              {new Date(l.time).toLocaleTimeString()}
            </span>
            <span className={cn('w-11 shrink-0 font-semibold', levelStyle[l.level])}>{l.level}</span>
            <span className="min-w-0 break-words">
              {l.message}
              {l.attrs && <span className="ml-2 text-subtle">{l.attrs}</span>}
            </span>
          </div>
        ))}
      </div>
      <p className="mt-2 text-[12px] text-subtle">{t('reports:dashboard.logLimit', { lines: fmt.number(maxLines) })}</p>
    </section>
  );
}
