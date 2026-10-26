import { useState } from 'react';
import { Switch } from '@/components/ui/switch';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';

/** Opt-in control for the shared Maintenance row, with explicit history-loss confirmation. */
export function EmptyTrashControl({ enabled, busy, onChange }: {
  enabled: boolean; busy: boolean; onChange: (enabled: boolean) => Promise<unknown>;
}) {
  const [confirm, setConfirm] = useState(false);
  const [error, setError] = useState<string>();
  const save = async (value: boolean) => {
    setError(undefined);
    try { await onChange(value); setConfirm(false); }
    catch (e) { setError(e instanceof Error ? e.message : 'Could not save this setting.'); }
  };
  return <>
    <Switch checked={enabled} disabled={busy} aria-label="Empty trash automatically" onCheckedChange={v => v ? setConfirm(true) : void save(false)} />
    {!confirm && error && <p role="alert" className="text-sm text-red-400">{error}</p>}
    <ConfirmDialog open={confirm} onOpenChange={v => { if (!busy) { setConfirm(v); setError(undefined); } }}
      title="Enable automatic empty trash?" confirmLabel="Enable automatic cleanup" destructive busy={busy} error={error} onConfirm={() => void save(true)}>
      <p>This permanently removes missing titles and their watch history after eligible scans, during maintenance. Leave it Off if you disconnect media drives.</p>
      <p>Arc checks every root and saves a safety backup first. An empty or unavailable root is skipped automatically; you can review and confirm a manual cleanup instead. After restarting Arc, a fresh complete scan is required.</p>
    </ConfirmDialog>
  </>;
}
