import { useTranslation } from 'react-i18next';
import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { ErrorNotice } from '@/components/Page';

/** The server's own time, which maintenance hours follow (a container is often on UTC, not local time). */
export function MaintenanceClock({ start, end }: { start: number; end: number }) {
  const { t } = useTranslation('serverSettings');
  const query = useQuery({ queryKey: ['maintenance-window', start, end], queryFn: api.maintenanceWindow, refetchInterval: 30000 });
  return (
    <span className="mt-1 block" aria-label={t("serverSettings:maintenanceClock.serverMaintenanceClock")}>
      <ErrorNotice error={query.error} onRetry={() => void query.refetch()} busy={query.isFetching} />
      {query.data && !query.error && <>{t('serverSettings:maintenanceClock.time', { time: query.data.local_time.slice(11, 16), zone: query.data.timezone })}</>}
    </span>
  );
}
