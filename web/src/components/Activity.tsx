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

/** The Activity button: spins while the server is busy, and opens the list of tasks. */
export function ActivityButton({ className }: { className?: string }) {
  const admin = useIsAdmin();
  const { data, refetch } = useActivity();
  const [open, setOpen] = useState(false);
  if (!admin) return null;
  const busy = (data?.running.length ?? 0) > 0;
  return (
    <Popover.Root
      open={open}
      onOpenChange={(o) => {
        setOpen(o);
        if (o) refetch();
      }}
    >
      <Popover.Trigger
        aria-label={busy ? `Activity: ${data!.running.length} running` : 'Activity'}
        className={cn(
          'relative flex size-8 items-center justify-center rounded-full text-muted outline-none transition-colors hover:bg-surface-3 hover:text-foreground focus-visible:ring-2 focus-visible:ring-accent',
          busy && 'text-accent',
          className,
        )}
      >
        {busy ? <Loader2 className="size-[18px] animate-spin" /> : <Zap className="size-[18px]" />}
        {busy && data!.running.length > 1 && (
          <span className="absolute -right-0.5 -top-0.5 flex size-4 items-center justify-center rounded-full bg-accent text-[10px] font-semibold text-white">
            {data!.running.length}
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
              <LayoutDashboard className="size-4" /> View Dashboard
            </Link>
          </Popover.Close>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}

/** The Activity menu, kept as simple as Plex's: what's running, how far along, and what just finished. */
function SimpleActivity() {
  const { data } = useActivity(true);
  if (!data) return <div className="h-20 animate-pulse rounded-xl bg-surface-2" />;
  const done = data.recent.slice(0, 4);
  return (
    <div className="space-y-5">
      {data.running.length === 0 ? (
        <p className="text-sm text-muted">Nothing running right now.</p>
      ) : (
        <ul className="space-y-4">
          {data.running.map((t) => (
            <SimpleTask key={t.id} task={t} />
          ))}
        </ul>
      )}
      {done.length > 0 && (
        <section>
          <h3 className="mb-2 text-[11px] font-semibold uppercase tracking-wider text-subtle">Recent</h3>
          <ul className="space-y-2">
            {done.map((t) => (
              <li key={t.id} className="flex items-start gap-2.5 text-sm">
                {t.error ? <AlertCircle className="mt-0.5 size-4 shrink-0 text-warning" /> : <Check className="mt-0.5 size-4 shrink-0 text-subtle" />}
                <div className="min-w-0 flex-1">
                  <div className="flex items-baseline gap-2">
                    <p className="min-w-0 flex-1 truncate">{finishedTitle(t.title)}</p>
                    <span className="shrink-0 text-xs text-subtle">{t.stopped ? 'Stopped' : since(t.finished_at!)}</span>
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
  const qc = useQueryClient();
  const stop = useMutation({ mutationFn: () => api.stopActivity(t.id), onSuccess: () => {
    void qc.invalidateQueries({ queryKey: ['activity'] });
    void qc.invalidateQueries({ queryKey: ['libraries'] });
  } });
  const pct = t.total ? Math.min(100, Math.round(((t.done ?? 0) / t.total) * 100)) : null;
  // One plain line: for scans, which of the three steps it's on (finding, matching, reading
  // files), then how far along it is or the title being worked on.
  const first = t.current?.[0]?.subject;
  const more = (t.current?.length ?? 0) - 1;
  const step = t.kind === 'scan' ? scanStep(t) : '';
  const progress = t.total ? `${t.done ?? 0} of ${t.total}` : first ? (more > 0 ? `${first} and ${more} more` : first) : '';
  const line = t.stopping || stop.isPending ? 'Stopping…' : [step, progress].filter(Boolean).join(' · ');
  return (
    <li>
      <div className="flex items-center gap-2 text-sm">
        <p className="min-w-0 flex-1 truncate font-medium">{t.title}</p>
        {pct !== null && <span className="shrink-0 text-xs tabular-nums text-subtle">{pct}%</span>}
        {t.can_stop && (
          <button
            type="button"
            aria-label={`Stop ${t.title}`}
            title="Stop"
            disabled={t.stopping || stop.isPending}
            onClick={() => stop.mutate()}
            className="flex size-6 shrink-0 items-center justify-center rounded-full text-subtle outline-none hover:bg-surface-3 hover:text-foreground focus-visible:ring-2 focus-visible:ring-accent disabled:opacity-50"
          >
            <X className="size-3.5" />
          </button>
        )}
      </div>
      {line && <p className="mt-0.5 truncate text-xs text-subtle">{line}</p>}
      <ProgressBar pct={pct} />
      <ErrorNotice error={stop.error} onRetry={() => stop.mutate()} busy={stop.isPending} />
    </li>
  );
}

/** The step a scan is on, in the words of scan → match → read files. */
function scanStep(t: ActivityTask) {
  const stage = t.current?.[0]?.stage ?? '';
  if (/^Inspecting/.test(stage)) return 'Reading files';
  if (stage) return 'Matching';
  const phase = (t.detail ?? '').split(' · ')[0];
  if (phase === 'Reading file details') return 'Reading files';
  if (/^(Matching|Getting show details|Updating)/.test(phase)) return 'Matching';
  if (phase === 'Loading alternate episode orders') return 'Getting episode orders';
  if (phase === 'Adding extras') return 'Adding extras';
  return 'Finding files';
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
  const { data } = useActivity(true);
  if (!data) return <div className="h-20 animate-pulse rounded-xl bg-surface-2" />;
  const done = data.recent.slice(0, recent);
  return (
    <div className="space-y-5">
      <section>
        <h3 className="mb-2 text-[11px] font-semibold uppercase tracking-wider text-subtle">Running</h3>
        {data.running.length === 0 ? (
          <p className="text-sm text-muted">Nothing right now. Scans, refreshes and other background work show here while they run.</p>
        ) : (
          <ul className="space-y-3.5">
            {data.running.map((t) => (
              <RunningTask key={t.id} task={t} />
            ))}
          </ul>
        )}
      </section>
      {data.scheduled?.some(t => t.enabled && t.pending > 0 && t.state !== 'running') && <section>
        <h3 className="mb-2 text-[11px] font-semibold uppercase tracking-wider text-subtle">Scheduled work</h3>
        <ul className="space-y-2">{data.scheduled.filter(t => t.enabled && t.pending > 0 && t.state !== 'running').map(t => <li key={t.kind} className="text-sm"><p>{t.title}</p><p className="text-xs text-subtle">{t.state} · {t.detail}</p></li>)}</ul>
      </section>}
      {done.length > 0 && (
        <section>
          <h3 className="mb-2 text-[11px] font-semibold uppercase tracking-wider text-subtle">Recently finished</h3>
          <ul className="space-y-2">
            {done.map((t) => (
              <li key={t.id} className="flex items-start gap-2.5 text-sm">
                {t.error ? <AlertCircle className="mt-0.5 size-4 shrink-0 text-warning" /> : <Check className="mt-0.5 size-4 shrink-0 text-subtle" />}
                <div className="min-w-0 flex-1">
                  <p className="break-words">{t.title}{t.stopped && <span className="ml-2 text-subtle">Stopped</span>}</p>
                  {t.detail && <p className="mt-0.5 break-words text-xs text-muted">{t.detail}</p>}
                  {t.error && <p className="mt-0.5 break-words text-xs text-warning">{t.error}</p>}
                  <p className="mt-0.5 text-xs text-subtle">{since(t.finished_at!)} · took {took(t)}</p>
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
      {t.stopping && <p role="status" className="mt-1 text-xs text-muted">Stopping… completed changes will be kept.</p>}
      {t.can_stop && <Button size="sm" variant="secondary" className="mt-2" disabled={t.stopping || stop.isPending} onClick={() => stop.mutate()} aria-label={`Stop ${t.title}`}>{t.stopping || stop.isPending ? 'Stopping…' : 'Stop'}</Button>}
      <ErrorNotice error={stop.error} onRetry={() => stop.mutate()} busy={stop.isPending} />
      {t.detail && t.detail !== t.title && <p className="mt-0.5 break-words text-xs text-subtle">{t.detail}</p>}
      {!t.detail && !!t.total && <p className="mt-0.5 text-xs text-subtle">{t.done ?? 0} of {t.total}</p>}
      <ProgressBar pct={pct} />
      {!!t.current?.length && <div className="mt-3 space-y-2 border-l-2 border-border pl-3">
        {t.current.slice(0, 2).map(work => <CurrentOperation key={work.id} work={work} />)}
        {t.current.length > 2 && <details>
          <summary className="cursor-pointer py-1 text-xs font-medium text-accent">Show {t.current.length - 2} more active {t.current.length === 3 ? 'operation' : 'operations'}</summary>
          <div className="mt-2 space-y-2">{t.current.slice(2).map(work => <CurrentOperation key={work.id} work={work} />)}</div>
        </details>}
      </div>}
    </li>
  );
}

function CurrentOperation({ work }: { work: ActivityOperation }) {
  return <div className="min-w-0 text-xs">
    <p className="break-words font-medium">{work.stage} · {work.subject}</p>
    {!!work.total && <p className="mt-0.5 text-subtle">{work.done ?? 0} of {work.total}</p>}
    {work.detail && <details className="mt-0.5">
      <summary className="cursor-pointer py-0.5 text-subtle">Details</summary>
      <p className="break-words py-1 text-muted">{work.detail}</p>
    </details>}
  </div>;
}

/** "just now", "4 min ago", "2 h ago". */
function since(iso: string) {
  const s = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000);
  if (s < 60) return 'just now';
  if (s < 3600) return `${Math.floor(s / 60)} min ago`;
  if (s < 86400) return `${Math.floor(s / 3600)} h ago`;
  return new Date(iso).toLocaleDateString('en-GB', { day: 'numeric', month: 'short' });
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
  if (ms < 1000) return 'under a second';
  const s = Math.round(ms / 1000);
  if (s < 60) return `${s}s`;
  if (s < 3600) return `${Math.floor(s / 60)}m ${s % 60}s`;
  return `${Math.floor(s / 3600)}h ${Math.floor((s % 3600) / 60)}m`;
}
