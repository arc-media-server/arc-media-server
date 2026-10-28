import { useTranslation } from 'react-i18next';
import { useState } from 'react';
import { Switch } from '@/components/ui/switch';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';

/** Opt-in control for the shared Maintenance row, with explicit history-loss confirmation. */
export function EmptyTrashControl({ enabled, busy, onChange }: {
  enabled: boolean; busy: boolean; onChange: (enabled: boolean) => Promise<unknown>;
}) {
  const { t } = useTranslation('serverSettings');
  const [confirm, setConfirm] = useState(false);
  const [error, setError] = useState<string>();
  const save = async (value: boolean) => {
    setError(undefined);
    try { await onChange(value); setConfirm(false); }
    catch (e) { setError(e instanceof Error ? e.message : t('serverSettings:emptyTrashTask.saveError')); }
  };
  return <>
    <Switch checked={enabled} disabled={busy} aria-label={t("serverSettings:emptyTrashTask.emptyTrashAutomatically")} onCheckedChange={v => v ? setConfirm(true) : void save(false)} />
    {!confirm && error && <p role="alert" className="text-sm text-red-400">{error}</p>}
    <ConfirmDialog open={confirm} onOpenChange={v => { if (!busy) { setConfirm(v); setError(undefined); } }}
      title={t("serverSettings:emptyTrashTask.enableAutomaticEmptyTrash")} confirmLabel={t("serverSettings:emptyTrashTask.enableAutomaticCleanup")} destructive busy={busy} error={error} onConfirm={() => void save(true)}>
      <p>{t("serverSettings:emptyTrashTask.afterEachCompleteScan")}</p>
      <p>{t("serverSettings:emptyTrashTask.itOnlyRunsWhen")}</p>
    </ConfirmDialog>
  </>;
}
