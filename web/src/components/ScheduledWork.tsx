import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { ErrorNotice } from '@/components/Page';

export function ScheduledWork() {
 const query = useQuery({ queryKey: ['scheduled-work'], queryFn: api.maintenance, refetchInterval: 3000 });
 const qc = useQueryClient();
 const run = useMutation({ mutationFn: api.runMaintenance, onSuccess: () => { qc.invalidateQueries({queryKey:['scheduled-work']});qc.invalidateQueries({queryKey:['activity']}); } });
 return <div className="border-t border-border px-5 py-4">
  <h3 className="font-medium">Task status</h3>
  <p className="mt-1 text-xs text-muted">Run now processes the current backlog outside these hours. Playback and scanning still take priority.</p>
  <ErrorNotice error={query.error} onRetry={()=>void query.refetch()} />
  <ErrorNotice error={run.error} onRetry={()=>run.variables && run.mutate(run.variables)} />
  <ul className="mt-3 divide-y divide-border">{query.data?.map(task=><li key={task.kind} className="flex items-center justify-between gap-3 py-3">
   <div className="min-w-0"><p className="text-sm font-medium">{task.title}</p><p role="status" className="text-xs text-muted">{task.state} · {task.detail}</p>
    {(task.pending>0 || task.completed>0 || task.failed>0) && <p className="text-xs text-subtle">{task.pending} queued · {task.completed} completed · {task.failed} failed</p>}
   </div>
   <Button size="sm" variant="secondary" aria-label={`Run ${task.title.toLowerCase()} now`} disabled={!task.enabled || run.isPending || task.state==='running'} onClick={()=>run.mutate(task.kind)}>Run now</Button>
  </li>)}</ul>
 </div>;
}
