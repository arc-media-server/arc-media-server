import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { ErrorNotice } from '@/components/Page';

/** The server's own time, which maintenance hours follow (a container is often on UTC, not local time). */
export function MaintenanceClock({ start, end }: { start: number; end: number }) {
  const query = useQuery({ queryKey: ['maintenance-window', start, end], queryFn: api.maintenanceWindow, refetchInterval: 30000 });
  return (
    <span className="mt-1 block" aria-label="Server maintenance clock">
      <ErrorNotice error={query.error} onRetry={() => void query.refetch()} busy={query.isFetching} />
      {query.data && !query.error && <>Server time: {query.data.local_time.slice(11, 16)} {query.data.timezone}</>}
    </span>
  );
}
