import * as fmt from '@/lib/format';
import { useTranslation } from 'react-i18next';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { ErrorNotice } from './Page';
import { Switch } from './ui/switch';
import { Button } from './ui/button';

export function Diagnostics() {
  const { t } = useTranslation('serverSettings');
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
      <span><span className="block text-sm font-medium">{t("serverSettings:diagnostics.detailedLogging")}</span><span className="mt-1 block text-xs text-muted">{t("serverSettings:diagnostics.recordExtraDetailFor")}</span></span>
      <Switch aria-label={t("serverSettings:diagnostics.detailedLogging")} checked={status.data?.detailed ?? false} disabled={!status.data || update.isPending} onCheckedChange={v => update.mutate(v)} />
    </label>
    {status.data?.until && <p className="text-xs text-muted">{t('serverSettings:diagnostics.endsAt', { time: fmt.localTime(new Date(status.data.until)) })}</p>}
    {status.data && <p className="text-xs text-muted">{status.data.persistent ? t("serverSettings:diagnostics.recentLogsSurviveRestarts") : t("serverSettings:diagnostics.recentLogsAreAvailable")}</p>}
    {status.data?.storage_error && <ErrorNotice error={t('serverSettings:diagnostics.retainError', { error: status.data.storage_error })} />}
    <Button variant="secondary" disabled={!status.data || download.isPending} onClick={() => download.mutate()}>{download.isPending ? t("serverSettings:diagnostics.preparingDiagnostics") : t("serverSettings:diagnostics.downloadDiagnostics")}</Button>
    <p className="text-xs text-muted">{t("serverSettings:diagnostics.recentLogsAndBuild")}</p>
    <div className="flex items-center justify-between gap-4 border-t border-border pt-4">
      <span><span className="block text-sm font-medium">{t("serverSettings:diagnostics.apiDocumentation")}</span><span className="mt-1 block text-xs text-muted">{t("serverSettings:diagnostics.everyRequestThisServer")}</span></span>
      <a href="/docs/" target="_blank" rel="noopener" className="inline-flex h-10 shrink-0 items-center rounded-xl bg-surface-2 px-4 text-sm font-medium hover:bg-surface-3">{t("serverSettings:diagnostics.openApiDocs")}</a>
    </div>
  </div>;
}
