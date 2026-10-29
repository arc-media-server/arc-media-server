import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api, type Library } from '@/lib/api';
import { useLibraries } from './Sidebar';
import { Button } from './ui/button';
import { Dialog, DialogContent } from './ui/dialog';
import { useTranslation } from 'react-i18next';
import { useEffect, useRef, useState } from 'react';
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

/** Explicit batch cleanup keeps the scanner's stronger automatic safety gate, without enabling it. */
export function EmptyTrashAllButton() {
  const { t } = useTranslation('serverSettings');
  const libraries = useLibraries();
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);
  const [confirmed, setConfirmed] = useState(false);
  const [results, setResults] = useState<TrashResult[]>([]);
  const stop = useRef(false);
  useEffect(() => { stop.current = false; return () => { stop.current = true; }; }, []);
  const run = useMutation({ mutationFn: async (targets: Library[]) => {
    stop.current = false;
    for (const library of targets) {
      if (stop.current) break;
      setResults(old => [...old.filter(r => r.id !== library.id), { id: library.id, name: library.name, state: 'running', removed: 0 }]);
      try {
        const result = await api.emptyTrashSafely(library.id);
        setResults(old => old.map(r => r.id === library.id ? { ...r, state: result.skipped ?? 'done', removed: result.removed } : r));
      } catch (e) {
        setResults(old => old.map(r => r.id === library.id ? { ...r, state: 'failed', error: e instanceof Error ? e.message : String(e) } : r));
      }
      void qc.invalidateQueries({ queryKey: ['libraries'] });
      void qc.invalidateQueries({ queryKey: ['activity'] });
    }
    for (const key of ['items', 'health', 'health-summary', 'featured', 'continue']) void qc.invalidateQueries({ queryKey: [key] });
  } });
  const retryIDs = new Set(results.filter(r => r.state !== 'done' && r.state !== 'running').map(r => r.id));
  const start = () => { setConfirmed(false); setResults([]); run.mutate(libraries.data ?? []); };
  return <>
    <Button variant="secondary" size="sm" disabled={!libraries.data?.length || run.isPending} onClick={() => { setOpen(true); setConfirmed(true); }}>{t('emptyTrashTask.runAll')}</Button>
    <ConfirmDialog open={open && confirmed} onOpenChange={value => { setOpen(value); if (!value) setConfirmed(false); }} title={t('emptyTrashTask.runAllTitle')} destructive confirmLabel={t('emptyTrashTask.runAll')} onConfirm={start}>
      <p>{t('emptyTrashTask.runAllExplanation')}</p>
      <p>{t('emptyTrashTask.runAllSafety')}</p>
    </ConfirmDialog>
    <Dialog open={open && !confirmed} onOpenChange={value => { if (!run.isPending) setOpen(value); }}>
      <DialogContent title={t('emptyTrashTask.runAllTitle')} description={t('emptyTrashTask.runAllSafety')} footer={<>
        {!run.isPending && retryIDs.size > 0 && <Button variant="secondary" onClick={() => run.mutate((libraries.data ?? []).filter(l => retryIDs.has(l.id)))}>{t('emptyTrashTask.retryIncomplete')}</Button>}
        <Button disabled={run.isPending} onClick={() => setOpen(false)}>{t('emptyTrashTask.close')}</Button>
      </>}>
        {run.isPending && <p role="status">{t('emptyTrashTask.runningAll')}</p>}
        <p role="status">{t('emptyTrashTask.removedTotal', { count: results.reduce((n, r) => n + r.removed, 0) })}</p>
        <p className="text-sm text-muted">{t('emptyTrashTask.skippedTotal', { count: results.filter(r => r.state === 'busy' || r.state === 'scan_required').length })} · {t('emptyTrashTask.failedTotal', { count: results.filter(r => r.state === 'failed').length })}</p>
        <ul className="space-y-3">{results.map(r => <li key={r.id} className="text-sm"><strong>{r.name}</strong><p className="text-muted">{r.state === 'done' ? t('emptyTrashTask.removedLibrary', { count: r.removed }) : r.state === 'busy' ? t('emptyTrashTask.busyLibrary') : r.state === 'scan_required' ? t('emptyTrashTask.scanRequired') : r.state === 'running' ? t('emptyTrashTask.runningLibrary') : r.error}</p></li>)}</ul>
      </DialogContent>
    </Dialog>
  </>;
}

type TrashResult = { id: number; name: string; state: 'running' | 'done' | 'busy' | 'scan_required' | 'failed'; removed: number; error?: string };
