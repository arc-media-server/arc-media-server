import { useEffect, useRef, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Popover } from 'radix-ui';
import { Link } from 'react-router';
import { AlertCircle, Check, LayoutDashboard, Loader2, Zap } from 'lucide-react';
import { api, type ActivityTask } from '@/lib/api';
import { useIsAdmin } from '@/lib/auth';
import { cn } from '@/lib/utils';

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
            <ActivityList />
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

/** Running tasks with their progress, then what finished recently. */
export function ActivityList({ recent = 6 }: { recent?: number }) {
  const { data } = useActivity(true);
  if (!data) return <div className="h-20 animate-pulse rounded-xl bg-surface-2" />;
  const done = data.recent.slice(0, recent);
  return (
    <div className="space-y-5">
      <section>
        <h3 className="mb-2 text-[11px] font-semibold uppercase tracking-wider text-subtle">Running</h3>
        {data.running.length === 0 ? (
          <p className="text-sm text-muted">Nothing right now. Scans, scrubbing previews and backups show here while they run.</p>
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
                  <p className="truncate">{t.title}</p>
                  <p className={cn('truncate text-xs', t.error ? 'text-warning' : 'text-subtle')}>
                    {t.error ?? [since(t.finished_at!), t.kind === 'scan' ? t.detail : `took ${took(t)}`].filter(Boolean).join(' · ')}
                  </p>
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
  const pct = t.total ? Math.min(100, Math.round(((t.done ?? 0) / t.total) * 100)) : null;
  return (
    <li>
      <div className="flex items-baseline justify-between gap-3 text-sm">
        <p className="truncate font-medium">{t.title}</p>
        {pct !== null && <span className="shrink-0 text-xs tabular-nums text-subtle">{pct}%</span>}
      </div>
      {t.detail && <p className="mt-0.5 truncate text-xs text-subtle">{t.detail}</p>}
      <div className="mt-2 h-1 overflow-hidden rounded-full bg-surface-3">
        <div
          className={cn('h-full rounded-full bg-accent transition-all duration-500', pct === null && 'w-1/3 animate-pulse')}
          style={pct === null ? undefined : { width: `${Math.max(pct, 2)}%` }}
        />
      </div>
    </li>
  );
}

/** "just now", "4 min ago", "2 h ago". */
function since(iso: string) {
  const s = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000);
  if (s < 60) return 'just now';
  if (s < 3600) return `${Math.floor(s / 60)} min ago`;
  if (s < 86400) return `${Math.floor(s / 3600)} h ago`;
  return new Date(iso).toLocaleDateString('en-GB', { day: 'numeric', month: 'short' });
}

function took(t: ActivityTask) {
  const s = Math.round((new Date(t.finished_at!).getTime() - new Date(t.started_at).getTime()) / 1000);
  if (s < 60) return `${Math.max(s, 1)}s`;
  if (s < 3600) return `${Math.floor(s / 60)}m ${s % 60}s`;
  return `${Math.floor(s / 3600)}h ${Math.floor((s % 3600) / 60)}m`;
}
