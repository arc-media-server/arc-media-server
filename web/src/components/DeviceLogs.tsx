import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { api } from '@/lib/api';
import * as fmt from '@/lib/format';
import { Button } from '@/components/ui/button';
import { inputClass } from '@/components/ui/field';

export function DeviceLogs() {
  const { t } = useTranslation('reports');
  const [chosen, setChosen] = useState<number>();
  const devices = useQuery({ queryKey: ['client-log-devices'], queryFn: api.clientLogDevices });
  const device = devices.data?.find(d => d.id === chosen) ?? devices.data?.[0];
  const id = device?.id;
  const batches = useQuery({
    queryKey: ['client-log-batches', id],
    queryFn: () => api.clientLogs(id!),
    enabled: id !== undefined,
    retry: false,
  });
  const text = batches.data?.map(batch => batch.entries.map(entry =>
    `${new Date(entry.time_ms).toISOString()} ${entry.level.toUpperCase()} ${entry.event}`,
  ).join('\n')).join('\n');

  const refresh = async () => {
    const updated = await devices.refetch();
    if (id !== undefined && updated.data?.some(d => d.id === id)) await batches.refetch();
  };

  const download = () => {
    const url = URL.createObjectURL(new Blob([text ?? ''], { type: 'text/plain;charset=utf-8' }));
    const link = document.createElement('a');
    link.href = url;
    link.download = 'arc-device-logs.txt';
    link.click();
    window.setTimeout(() => URL.revokeObjectURL(url), 30_000);
  };

  return (
    <section>
      <div className="mb-3 flex flex-wrap items-center gap-3">
        <h2 className="mr-auto text-lg font-semibold">{t('deviceLogs.title')}</h2>
        <Button variant="secondary" size="sm" onClick={() => void refresh()} disabled={devices.isFetching || batches.isFetching}>{t('deviceLogs.refresh')}</Button>
        <Button variant="secondary" size="sm" onClick={download} disabled={!text || batches.isError}>{t('deviceLogs.download')}</Button>
      </div>
      {devices.isError || batches.isError ? (
        <p role="alert" className="text-sm text-warning">{t('deviceLogs.failed')}</p>
      ) : devices.isPending || (device && batches.isPending) ? (
        <p role="status" className="text-sm text-muted">{t('deviceLogs.loading')}</p>
      ) : !device ? (
        <p className="text-sm text-muted">{t('deviceLogs.empty')}</p>
      ) : null}
      {device && (
        <label className="my-3 flex flex-wrap items-center gap-2 text-sm">
          {t('deviceLogs.device')}
          <select className={inputClass + ' w-auto max-w-full'} value={device.id} onChange={e => setChosen(Number(e.target.value))}>
            {devices.data?.map(d => <option key={d.id} value={d.id}>{t('deviceLogs.option', { name: d.device, time: fmt.dateTime(new Date(d.received_at * 1000).toISOString()) })}</option>)}
          </select>
        </label>
      )}
      {text && !batches.isError && <pre tabIndex={0} className="max-h-[480px] overflow-auto whitespace-pre-wrap break-words rounded-2xl bg-surface p-3 text-xs ring-1 ring-border">{text}</pre>}
    </section>
  );
}
