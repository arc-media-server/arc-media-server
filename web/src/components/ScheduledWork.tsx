import type { ReactNode } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api, type ScheduledTask } from '@/lib/api';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { ErrorNotice } from '@/components/Page';

/** One background task: its setting and its live status share a row, so each task is listed once. */
export type MaintenanceTaskRow = { kind: string; label: string; description: string; control: ReactNode; feedback?: ReactNode; action?: ReactNode };

// Routine states (scheduled, waiting, up to date) aren't shown: only what needs a glance.
const states: Record<string, { label: string; tone: string }> = {
  running: { label: 'Running', tone: 'bg-accent animate-pulse' },
  paused: { label: 'Paused while something plays or scans', tone: 'bg-warning' },
  failed: { label: 'Some items failed and will be tried again', tone: 'bg-warning' },
};

export function MaintenanceTasks({ rows }: { rows: MaintenanceTaskRow[] }) {
  const query = useQuery({ queryKey: ['scheduled-work'], queryFn: api.maintenance, refetchInterval: 3000 });
  const qc = useQueryClient();
  const run = useMutation({ mutationFn: api.runMaintenance, onSuccess: () => { qc.invalidateQueries({ queryKey: ['scheduled-work'] }); qc.invalidateQueries({ queryKey: ['activity'] }); } });
  return (
    <div>
      <div className="px-4 pt-4">
        <h3 className="font-medium">Tasks</h3>
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
                {row.feedback && <div className="mt-2 empty:hidden">{row.feedback}</div>}
              </div>
              <div className="flex w-full items-center gap-3 sm:w-auto">
                {row.control}
                {row.action ?? <Button
                  size="sm"
                  variant="secondary"
                  className="ml-auto shrink-0 sm:ml-0"
                  aria-label={`Run ${row.label} now`}
                  disabled={!task?.enabled || run.isPending || task.state === 'running'}
                  onClick={() => run.mutate(row.kind)}
                >
                  Run now
                </Button>}
              </div>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

function TaskStatus({ task }: { task?: ScheduledTask }) {
  const state = task && states[task.state];
  if (!task || !state) return null;
  return (
    <p role="status" className="mt-1.5 flex flex-wrap items-center gap-x-2 text-xs text-subtle">
      <span className="flex items-center gap-1.5 font-medium text-muted">
        <span aria-hidden className={cn('size-1.5 rounded-full', state.tone)} />
        {state.label}
      </span>
      {task.state === 'running' && task.detail && <span>· {task.detail}</span>}
    </p>
  );
}
