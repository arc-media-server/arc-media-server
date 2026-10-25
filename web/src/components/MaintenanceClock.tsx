import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { ErrorNotice } from '@/components/Page';

export function MaintenanceClock({ start, end }: { start: number; end: number }) {
  const query = useQuery({ queryKey: ['maintenance-window', start, end], queryFn: api.maintenanceWindow, refetchInterval: 30000 });
  return <div className="space-y-1 px-4 py-3 text-xs text-subtle" aria-label="Server maintenance clock">
    <ErrorNotice error={query.error} onRetry={() => void query.refetch()} busy={query.isFetching} />
    {!query.data && query.isPending && <p role="status">Loading server time…</p>}
    {query.data && !query.error && <>
      <p>Server time: {query.data.local_time} · {query.data.timezone}</p>
      <p>{query.data.all_day ? 'Maintenance window: open all day.' : query.data.open ? 'Maintenance window is open now.' : `Next maintenance window: ${query.data.next_local ?? 'unavailable'}.`}</p>
      <p>Tasks still follow their enable switches and wait for a quiet server.</p>
    </>}
  </div>;
}
