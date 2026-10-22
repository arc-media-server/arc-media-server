import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { ErrorNotice } from './Page';
import { Switch } from './ui/switch';
import { Button } from './ui/button';

export function Diagnostics() {
  const qc = useQueryClient();
  const status = useQuery({ queryKey: ['diagnostics'], queryFn: api.diagnostics, refetchInterval: 10000 });
  const update = useMutation({ mutationFn: api.setDiagnostics, onSuccess: data => qc.setQueryData(['diagnostics'], data) });
  const download = useMutation({ mutationFn: api.downloadDiagnostics, onSuccess: blob => {
    const url = URL.createObjectURL(blob), a = document.createElement('a'); a.href = url; a.download = 'arc-diagnostics.zip'; a.click(); window.setTimeout(() => URL.revokeObjectURL(url), 30000);
  } });
  return <div className="space-y-4 p-4">
    <ErrorNotice error={status.error} onRetry={() => void status.refetch()} />
    <ErrorNotice error={download.error} onRetry={() => download.mutate()} />
    <ErrorNotice error={update.error} onRetry={() => { if (update.variables !== undefined) update.mutate(update.variables); }} />
    <label className="flex items-center justify-between gap-4">
      <span><span className="block text-sm font-medium">Detailed logging</span><span className="mt-1 block text-xs text-muted">Record extra detail for troubleshooting. Turns off automatically after 30 minutes.</span></span>
      <Switch aria-label="Detailed logging" checked={status.data?.detailed ?? false} disabled={!status.data || update.isPending} onCheckedChange={v => update.mutate(v)} />
    </label>
    {status.data?.until && <p className="text-xs text-muted">Detailed logging ends at {new Date(status.data.until).toLocaleTimeString()}.</p>}
    {status.data && <p className="text-xs text-muted">{status.data.persistent ? 'Recent logs survive restarts. Arc keeps up to three 2 MB log files.' : 'Recent logs are available for this session only.'}</p>}
    {status.data?.storage_error && <ErrorNotice error={'Could not retain logs: ' + status.data.storage_error} />}
    <Button variant="secondary" disabled={!status.data || download.isPending} onClick={() => download.mutate()}>{download.isPending ? 'Preparing diagnostics…' : 'Download diagnostics'}</Button>
    <p className="text-xs text-muted">Includes recent logs and build information. Known secrets are removed; profile/media names and paths may be included. No database or media files.</p>
  </div>;
}
