import i18n from '@/i18n';
import { shortDate } from '@/lib/format';
import { useTranslation } from 'react-i18next';
import { useEffect, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Popover } from 'radix-ui';
import { Link } from 'react-router';
import { AlertCircle, Check, LayoutDashboard, Loader2, X, Zap } from 'lucide-react';
import { api, type ActivityOperation, type ActivityTask } from '@/lib/api';
import { useIsAdmin } from '@/lib/auth';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { ErrorNotice } from '@/components/Page';

/** What the server is doing (admins only), checked often while something runs or the list is open. */
export function useActivity(watching = false) {
  const admin = useIsAdmin();
  const qc = useQueryClient();
  const query = useQuery({
    queryKey: ['activity'],
    queryFn: api.activity,
    enabled: admin,
    refetchInterval: (q) => (watching || q.state.data?.running.length ? 2000 : 10000),
  });
  // When a task finishes, what it changed (new titles, details) should show up.
  const running = query.data?.running.length ?? 0;
  const before = useRef(running);
  useEffect(() => {
    if (running < before.current) {
      for (const key of ['libraries', 'items', 'featured', 'recent', 'backups']) qc.invalidateQueries({ queryKey: [key] });
    }
    before.current = running;
  }, [running, qc]);
  return query;
}

// Background tasks that give way to playback and scans (intro detection, previews, media
// analysis) can wait without doing anything; like Plex, the menu lists them only while they
// are working on an item. The Dashboard still shows everything.
const waitsForTurn = new Set(['markers', 'previews', 'analysis']);
function working(running: ActivityTask[]) {
  return running.filter((t) => !waitsForTurn.has(t.kind) || !!t.current?.length || t.stopping);
}

/** The Activity button: spins while the server is busy, and opens the list of tasks. */
export function ActivityButton({ className }: { className?: string }) {
  const { t: tr } = useTranslation('common');
  const admin = useIsAdmin();
  const { data, refetch } = useActivity();
  const [open, setOpen] = useState(false);
  if (!admin) return null;
  const active = working(data?.running ?? []);
  const busy = active.length > 0;
  return (
    <Popover.Root
      open={open}
      onOpenChange={(o) => {
        setOpen(o);
        if (o) refetch();
      }}
    >
      <Popover.Trigger
        aria-label={busy ? tr('activity.runningCount', { count: active.length }) : tr('activity.label')}
        className={cn(
          'relative flex size-8 items-center justify-center rounded-full text-muted outline-none transition-colors hover:bg-surface-3 hover:text-foreground focus-visible:ring-2 focus-visible:ring-accent',
          busy && 'text-accent',
          className,
        )}
      >
        {busy ? <Loader2 className="size-[18px] animate-spin" /> : <Zap className="size-[18px]" />}
        {active.length > 1 && (
          <span className="absolute -right-0.5 -top-0.5 flex size-4 items-center justify-center rounded-full bg-accent text-[10px] font-semibold text-white">
            {active.length}
          </span>
        )}
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          align="start"
          sideOffset={8}
          collisionPadding={12}
          className="animate-menu-in z-50 w-[340px] bg-surface max-w-[calc(100vw-24px)] overflow-hidden rounded-2xl border border-border shadow-2xl shadow-black/40"
        >
          <div className="max-h-[min(70vh,520px)] overflow-y-auto p-4">
            <SimpleActivity />
          </div>
          <Popover.Close asChild>
            <Link to="/dashboard" className="flex min-h-11 items-center gap-2 border-t border-border px-4 py-3 text-sm font-medium text-accent hover:bg-surface-2 focus-visible:outline focus-visible:outline-2 focus-visible:outline-accent">
              <LayoutDashboard className="size-4" />{' '}{tr('activity.viewDashboard')}</Link>
          </Popover.Close>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}

/** The Activity menu, kept as simple as Plex's: what's running, how far along, and what just finished. */
function SimpleActivity() {
  const { t: tr } = useTranslation('common');
  const { data } = useActivity(true);
  if (!data) return <div className="h-20 animate-pulse rounded-xl bg-surface-2" />;
  const done = data.recent.slice(0, 4);
  const active = working(data.running);
  return (
    <div className="space-y-5">
      {active.length === 0 ? (
        <p className="text-sm text-muted">{tr('activity.idle')}</p>
      ) : (
        <ul className="space-y-4">
          {active.map((t) => (
            <SimpleTask key={t.id} task={t} />
          ))}
        </ul>
      )}
      {done.length > 0 && (
        <section>
          <h3 className="mb-2 text-[11px] font-semibold uppercase tracking-wider text-subtle">{tr('activity.recent')}</h3>
          <ul className="space-y-2">
            {done.map((t) => (
              <li key={t.id} className="flex items-start gap-2.5 text-sm">
                {t.error ? <AlertCircle className="mt-0.5 size-4 shrink-0 text-warning" /> : <Check className="mt-0.5 size-4 shrink-0 text-subtle" />}
                <div className="min-w-0 flex-1">
                  <div className="flex items-baseline gap-2">
                    <p className="min-w-0 flex-1 truncate">{finishedTitle(t.title)}</p>
                    <span className="shrink-0 text-xs text-subtle">{t.stopped ? tr('activity.stopped') : since(t.finished_at!)}</span>
                  </div>
                  {t.error && <p className="mt-0.5 break-words text-xs text-warning">{t.error}</p>}
                </div>
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}

function SimpleTask({ task: t }: { task: ActivityTask }) {
  const { t: tr } = useTranslation('common');
  const qc = useQueryClient();
  const stop = useMutation({ mutationFn: () => api.stopActivity(t.id), onSuccess: () => {
    void qc.invalidateQueries({ queryKey: ['activity'] });
    void qc.invalidateQueries({ queryKey: ['libraries'] });
  } });
  const pct = t.total ? Math.min(100, Math.round(((t.done ?? 0) / t.total) * 100)) : null;
  // Scans show one line per step that's under way (Scanning, Matching, Analysing), each with the
  // one item it's on. Other tasks get one plain line: the title being worked on (e.g. "Grey's
  // Anatomy · Season 4" for intro detection), or N of M.
  const stopping = t.stopping || stop.isPending;
  const first = t.current?.[0]?.subject;
  const more = (t.current?.length ?? 0) - 1;
  const steps = t.kind === 'scan' && !stopping ? scanSteps(t) : [];
  const line = stopping ? tr('actions.stopping') : first ? (more > 0 ? tr('activity.moreSubjects', { subject: first, count: more }) : first) : t.total ? i18n.t('common:activity.progress', { done: t.done ?? 0, total: t.total }) : '';
  return (
    <li>
      <div className="flex items-center gap-2 text-sm">
        <p className="min-w-0 flex-1 truncate font-medium">{t.title}</p>
        {pct !== null && <span className="shrink-0 text-xs tabular-nums text-subtle">{pct}%</span>}
        {t.can_stop && (
          <button
            type="button"
            aria-label={tr('activity.stopTask', { title: t.title })}
            title={tr('actions.stop')}
            disabled={t.stopping || stop.isPending}
            onClick={() => stop.mutate()}
            className="flex size-6 shrink-0 items-center justify-center rounded-full text-subtle outline-none hover:bg-surface-3 hover:text-foreground focus-visible:ring-2 focus-visible:ring-accent disabled:opacity-50"
          >
            <X className="size-3.5" />
          </button>
        )}
      </div>
      {steps.length > 0 ? (
        <dl className="mt-1 space-y-0.5 text-xs">
          {steps.map(([step, item]) => (
            <div key={step} className="flex min-w-0 gap-1.5">
              <dt className="shrink-0 text-muted">{step}</dt>
              {item && <dd className="min-w-0 truncate text-subtle">{item}</dd>}
            </div>
          ))}
        </dl>
      ) : line && <p className="mt-0.5 truncate text-xs text-subtle">{line}</p>}
      <ProgressBar pct={pct} />
      <ErrorNotice error={stop.error} onRetry={() => stop.mutate()} busy={stop.isPending} />
    </li>
  );
}

/** A scan's steps under way (Scanning, Matching, Analysing), each with the one item it's on. */
function scanSteps(t: ActivityTask): [string, string][] {
  const ops = t.current ?? [];
  const on = (stage: RegExp) => ops.find((o) => stage.test(o.stage))?.subject;
  const phase = (t.detail ?? '').split(' · ')[0];
  const count = t.total ? i18n.t('common:activity.progress', { done: t.done ?? 0, total: t.total }) : '';
  const steps: [string, string][] = [];
  const file = on(/^Checking file$/);
  const walking = /^(Opening library|Checking library folders|Checking files and titles|Looking through folders)$/.test(phase) || phase === '';
  if (file || walking) {
    const found = /(\d+) files/.exec(t.detail ?? '')?.[1];
    steps.push([i18n.t('common:activity.scanning'), file ? file.replace(/\.[^.]+$/, '') : found ? i18n.t('common:activity.filesFound', { count: Number(found) }) : '']);
  }
  const title = on(/^(Matching|Fetching|Saving|Linking|Checking episode|Mapping)/);
  if (title) steps.push([i18n.t('common:activity.matching'), title]);
  else if (/^(Matching|Getting show details|Updating|Loading alternate episode orders)/.test(phase)) steps.push([i18n.t('common:activity.matching'), count]);
  const reading = on(/^Inspecting/);
  if (reading) steps.push([i18n.t('common:activity.analysing'), reading]);
  else if (phase === 'Reading file details') steps.push([i18n.t('common:activity.analysing'), count]);
  return steps.length ? steps : [[i18n.t('common:activity.scanning'), '']];
}

function ProgressBar({ pct }: { pct: number | null }) {
  return (
    <div className="mt-2 h-1 overflow-hidden rounded-full bg-surface-3">
      <div
        className={cn('h-full rounded-full bg-accent transition-all duration-500', pct === null && 'w-1/3 animate-pulse')}
        style={pct === null ? undefined : { width: `${pct}%` }}
      />
    </div>
  );
}

/** Everything about running and recent tasks, for the Dashboard: each title's step, files and summaries. */
export function ActivityList({ recent = 6 }: { recent?: number }) {
  const { t: tr } = useTranslation('common');
  const { data } = useActivity(true);
  if (!data) return <div className="h-20 animate-pulse rounded-xl bg-surface-2" />;
  const done = data.recent.slice(0, recent);
  return (
    <div className="space-y-5">
      <section>
        <h3 className="mb-2 text-[11px] font-semibold uppercase tracking-wider text-subtle">{tr('activity.running')}</h3>
        {data.running.length === 0 ? (
          <p className="text-sm text-muted">{tr('activity.emptyDetail')}</p>
        ) : (
          <ul className="space-y-3.5">
            {data.running.map((t) => (
              <RunningTask key={t.id} task={t} />
            ))}
          </ul>
        )}
      </section>
      {data.scheduled?.some(t => t.enabled && t.pending > 0 && t.state !== 'running') && <section>
        <h3 className="mb-2 text-[11px] font-semibold uppercase tracking-wider text-subtle">{tr('activity.scheduled')}</h3>
        <ul className="space-y-2">{data.scheduled.filter(t => t.enabled && t.pending > 0 && t.state !== 'running').map(t => <li key={t.kind} className="text-sm"><p>{t.title}</p><p className="text-xs text-subtle">{t.state} · {t.detail}</p></li>)}</ul>
      </section>}
      {done.length > 0 && (
        <section>
          <h3 className="mb-2 text-[11px] font-semibold uppercase tracking-wider text-subtle">{tr('activity.finished')}</h3>
          <ul className="space-y-2">
            {done.map((t) => (
              <li key={t.id} className="flex items-start gap-2.5 text-sm">
                {t.error ? <AlertCircle className="mt-0.5 size-4 shrink-0 text-warning" /> : <Check className="mt-0.5 size-4 shrink-0 text-subtle" />}
                <div className="min-w-0 flex-1">
                  <p className="break-words">{t.title}{t.stopped && <span className="ml-2 text-subtle">{tr('activity.stopped')}</span>}</p>
                  {t.detail && <p className="mt-0.5 break-words text-xs text-muted">{t.detail}</p>}
                  {t.error && <p className="mt-0.5 break-words text-xs text-warning">{t.error}</p>}
                  <p className="mt-0.5 text-xs text-subtle">{tr('activity.elapsed', { since: since(t.finished_at!), duration: took(t) })}</p>
                </div>
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}

function RunningTask({ task: t }: { task: ActivityTask }) {
  const { t: tr } = useTranslation('common');
  const qc = useQueryClient();
  const stop = useMutation({ mutationFn: () => api.stopActivity(t.id), onSuccess: () => {
    void qc.invalidateQueries({ queryKey: ['activity'] });
    void qc.invalidateQueries({ queryKey: ['libraries'] });
  } });
  const pct = t.total ? Math.min(100, Math.round(((t.done ?? 0) / t.total) * 100)) : null;
  return (
    <li>
      <div className="flex items-baseline justify-between gap-3 text-sm">
        <p className="min-w-0 break-words font-medium">{t.title}</p>
        {pct !== null && <span className="shrink-0 text-xs tabular-nums text-subtle">{pct}%</span>}
      </div>
      {t.stopping && <p role="status" className="mt-1 text-xs text-muted">{tr('activity.stoppingDetail')}</p>}
      {t.can_stop && <Button size="sm" variant="secondary" className="mt-2" disabled={t.stopping || stop.isPending} onClick={() => stop.mutate()} aria-label={tr('activity.stopTask', { title: t.title })}>{t.stopping || stop.isPending ? tr('actions.stopping') : tr('actions.stop')}</Button>}
      <ErrorNotice error={stop.error} onRetry={() => stop.mutate()} busy={stop.isPending} />
      {t.detail && t.detail !== t.title && <p className="mt-0.5 break-words text-xs text-subtle">{t.detail}</p>}
      {!t.detail && !!t.total && <p className="mt-0.5 text-xs text-subtle">{tr('activity.progress', { done: t.done ?? 0, total: t.total })}</p>}
      <ProgressBar pct={pct} />
      {!!t.current?.length && <div className="mt-3 space-y-2 border-l-2 border-border pl-3">
        {t.current.slice(0, 2).map(work => <CurrentOperation key={work.id} work={work} />)}
        {t.current.length > 2 && <details>
          <summary className="cursor-pointer py-1 text-xs font-medium text-accent">{tr('activity.moreOperations', { count: t.current.length - 2 })}</summary>
          <div className="mt-2 space-y-2">{t.current.slice(2).map(work => <CurrentOperation key={work.id} work={work} />)}</div>
        </details>}
      </div>}
    </li>
  );
}

function CurrentOperation({ work }: { work: ActivityOperation }) {
  const { t: tr } = useTranslation('common');
  return <div className="min-w-0 text-xs">
    <p className="break-words font-medium">{work.stage} · {work.subject}</p>
    {!!work.total && <p className="mt-0.5 text-subtle">{tr('activity.progress', { done: work.done ?? 0, total: work.total })}</p>}
    {work.detail && <details className="mt-0.5">
      <summary className="cursor-pointer py-0.5 text-subtle">{tr('activity.details')}</summary>
      <p className="break-words py-1 text-muted">{work.detail}</p>
    </details>}
  </div>;
}

/** "just now", "4 min ago", "2 h ago". */
function since(iso: string) {
  const s = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000);
  if (s < 60) return i18n.t('common:time.justNow');
  if (s < 3600) return i18n.t('common:time.minutesAgo', { count: Math.floor(s / 60) });
  if (s < 86400) return i18n.t('common:time.hoursAgo', { count: Math.floor(s / 3600) });
  return shortDate(iso);
}

/** "Scanning Movies" reads as still running in a list of finished work: "Scanned Movies". */
const pastTense: Record<string, string> = {
  Scanning: 'Scanned', Refreshing: 'Refreshed', Reanalyzing: 'Reanalyzed', Analyzing: 'Analyzed', Updating: 'Updated',
  Finding: 'Found', Making: 'Made', Generating: 'Generated', Matching: 'Matched', Checking: 'Checked', Importing: 'Imported',
  Optimizing: 'Optimized', Converting: 'Converted', Restoring: 'Restored', Downloading: 'Downloaded', Creating: 'Created',
  Removing: 'Removed', Emptying: 'Emptied', Fetching: 'Fetched', Loading: 'Loaded', Preparing: 'Prepared',
};
export function finishedTitle(title: string) {
  if (title.startsWith('Backing up')) return 'Backed up' + title.slice('Backing up'.length);
  const [first, ...rest] = title.split(' ');
  return pastTense[first] ? [pastTense[first], ...rest].join(' ') : title;
}

function took(t: ActivityTask) {
  const ms = new Date(t.finished_at!).getTime() - new Date(t.started_at).getTime();
  if (ms < 1000) return i18n.t('common:time.underSecond');
  const s = Math.round(ms / 1000);
  if (s < 60) return i18n.t('common:time.seconds', { count: s });
  if (s < 3600) return i18n.t('common:time.minutesSeconds', { minutes: Math.floor(s / 60), seconds: s % 60 });
  return i18n.t('common:time.hoursMinutes', { hours: Math.floor(s / 3600), minutes: Math.floor((s % 3600) / 60) });
}
