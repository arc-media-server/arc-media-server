import * as fmt from '@/lib/format';
import { useTranslation } from 'react-i18next';
import { useEffect, useState, type ReactNode } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api, type ScheduledTask } from '@/lib/api';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { ErrorNotice } from '@/components/Page';

/** One background task: its setting and its live status share a row, so each task is listed once. */
export type MaintenanceTaskRow = { kind: string; label: string; description: string; control: ReactNode; feedback?: ReactNode; action?: ReactNode };

// Show accepted requests and actionable states; routine polling adds no log records.


export function MaintenanceTasks({ rows }: { rows: MaintenanceTaskRow[] }) {
  const { t } = useTranslation('serverSettings');
  const query = useQuery({ queryKey: ['scheduled-work'], queryFn: api.maintenance, refetchInterval: 3000 });
  const qc = useQueryClient();
  const [accepted, setAccepted] = useState<{kind: string; at: number}>();
  const run = useMutation({ mutationFn: api.runMaintenance, onMutate: () => setAccepted(undefined), onSuccess: (_, kind) => { setAccepted({kind,at: Date.now()}); qc.invalidateQueries({ queryKey: ['scheduled-work'] }); qc.invalidateQueries({ queryKey: ['activity'] }); } });
  useEffect(() => {
    if (!accepted) return;
    const task = query.data?.find(t => t.kind === accepted.kind);
    if (task?.requested || task?.state === 'running' || (query.dataUpdatedAt > accepted.at && (task?.last_run?.finished_at ?? 0) >= Math.floor(accepted.at / 1000))) setAccepted(undefined);
  }, [accepted, query.data, query.dataUpdatedAt]);
  return (
    <div>
      <div className="px-4 pt-4">
        <h3 className="font-medium">{t("serverSettings:scheduledWork.tasks")}</h3>
        <ErrorNotice error={query.error} onRetry={() => void query.refetch()} />
        <ErrorNotice error={run.error} onRetry={() => run.variables && run.mutate(run.variables)} />
      </div>
      <ul className="mt-2 divide-y divide-border">
        {rows.map((row) => {
          const task = query.data?.find((t) => t.kind === row.kind);
          return (
            <li key={row.kind} aria-label={row.label} className="flex flex-wrap items-center gap-x-6 gap-y-3 px-4 py-3.5">
              <div className="min-w-0 flex-1 basis-64">
                <p className="text-sm font-medium">{row.label}</p>
                <p className="mt-0.5 text-xs leading-relaxed text-muted">{row.description}</p>
                <TaskStatus task={task} />
                {accepted?.kind===row.kind && !task?.requested && task?.state!=='running' && <p role="status" className="mt-1 text-xs text-muted">{t('serverSettings:scheduledWork.accepted')}</p>}
                {row.feedback && <div className="mt-2 empty:hidden">{row.feedback}</div>}
              </div>
              <div className="flex w-full items-center gap-3 sm:w-auto">
                {row.control}
                {row.action ?? <Button
                  size="sm"
                  variant="secondary"
                  className="ml-auto shrink-0 sm:ml-0"
                  aria-label={t('serverSettings:scheduledWork.runTask', { task: row.label })}
                  disabled={!task?.enabled || run.isPending || task.state === 'running' || task.requested}
                  onClick={() => run.mutate(row.kind)}
                >{t("serverSettings:scheduledWork.runNow")}</Button>}
              </div>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

function TaskStatus({ task }: { task?: ScheduledTask }) {
  const { t } = useTranslation('serverSettings');
  const states: Record<string, { label: string; tone: string }> = {
  queued: { label: t('serverSettings:scheduledWork.queued'), tone: 'bg-subtle' },
  running: { label: t('serverSettings:scheduledWork.running'), tone: 'bg-accent animate-pulse' },
  paused: { label: t('serverSettings:scheduledWork.paused'), tone: 'bg-warning' },
  stopped: { label: t('serverSettings:scheduledWork.stopped'), tone: 'bg-subtle' },
  failed: { label: t('serverSettings:scheduledWork.failed'), tone: 'bg-warning' },
};
  const state = task && (task.state !== 'queued' || task.requested) && states[task.state];
  const last = task?.last_run && task.last_run.finished_at > 0 ? task.last_run : undefined;
  if (!task || (!state && !last)) return null;
  return (
    <div>
      {state && <p role="status" className="mt-1.5 flex flex-wrap items-center gap-x-2 text-xs text-subtle">
        <span className="flex items-center gap-1.5 font-medium text-muted">
          <span aria-hidden className={cn('size-1.5 rounded-full', state.tone)} />
          {state.label}
        </span>
        {task.detail && <span>· {task.detail}</span>}
      </p>}
      {last && <p className="mt-1 text-xs leading-relaxed text-muted">{t('serverSettings:scheduledWork.result', { time: fmt.localDateTime(new Date(last.finished_at * 1000)), detail: last.detail })}
        {last.error && <> · {last.error}</>}
      </p>}
    </div>
  );
}
