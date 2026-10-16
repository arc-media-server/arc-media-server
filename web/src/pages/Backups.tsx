import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArchiveRestore, CheckCircle2, DatabaseBackup, Hand, History, Loader2, Plus, ShieldCheck, Trash2 } from 'lucide-react';
import { api, type Backup, type BackupReason } from '@/lib/api';
import * as fmt from '@/lib/format';
import { EmptyState, ErrorState, Page, PageHeader } from '@/components/Page';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';

const reasons: Record<BackupReason, { label: string; icon: typeof History }> = {
  auto: { label: 'Daily backup', icon: History },
  manual: { label: 'Backed up by you', icon: Hand },
  'before-restore': { label: 'Saved before a restore', icon: ArchiveRestore },
  'before-upgrade': { label: 'Saved before an update', icon: ShieldCheck },
};

export function BackupsPage() {
  const qc = useQueryClient();
  const backups = useQuery({ queryKey: ['backups'], queryFn: api.backups });
  const [restoring, setRestoring] = useState<Backup | null>(null);
  const [deleting, setDeleting] = useState<Backup | null>(null);
  const [restored, setRestored] = useState<Backup | null>(null);

  const create = useMutation({
    mutationFn: api.createBackup,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['backups'] }),
  });
  const restore = useMutation({
    mutationFn: (b: Backup) => api.restoreBackup(b.name),
    onSuccess: (_, b) => {
      setRestoring(null);
      setRestored(b);
      // Everything may have changed: refetch it all.
      return qc.invalidateQueries();
    },
  });
  const remove = useMutation({
    mutationFn: (b: Backup) => api.deleteBackup(b.name),
    onSuccess: () => {
      setDeleting(null);
      return qc.invalidateQueries({ queryKey: ['backups'] });
    },
  });

  if (backups.error) return <ErrorState error={backups.error} />;

  return (
    <Page className="pb-16">
      <PageHeader
        title="Backups"
        subtitle="Arc backs up its database every day. Your media files are never part of a backup and are never changed."
        actions={
          <Button variant="secondary" size="sm" onClick={() => create.mutate()} disabled={create.isPending}>
            {create.isPending ? <Loader2 className="animate-spin" /> : <Plus />} Back Up Now
          </Button>
        }
      />
      <div className="space-y-4 px-6 md:px-10">
        {restored && (
          <p className="flex items-center gap-2 rounded-2xl bg-accent/10 px-4 py-3 text-sm ring-1 ring-accent/25">
            <CheckCircle2 className="size-4 shrink-0 text-accent" />
            Restored the backup from {fmt.dateTime(restored.created_at)}. Your libraries are being rescanned.
          </p>
        )}
        {create.error && <p className="text-sm text-warning">Couldn't back up: {create.error.message}</p>}

        {backups.data && backups.data.backups.length === 0 ? (
          <EmptyState icon={<DatabaseBackup />} title="No backups yet">
            The first one is made a minute after Arc starts, then once a day.
          </EmptyState>
        ) : (
          <div className="divide-y divide-border overflow-hidden rounded-2xl bg-surface ring-1 ring-border">
            {!backups.data && <div className="h-40 animate-pulse" />}
            {backups.data?.backups.map((b, i) => (
              <BackupRow key={b.name} backup={b} latest={i === 0} onRestore={() => setRestoring(b)} onDelete={() => setDeleting(b)} />
            ))}
          </div>
        )}

        {backups.data && (
          <p className="max-w-3xl text-xs leading-relaxed text-subtle">
            Arc keeps the last 7 daily backups plus one a week for the 4 weeks before that. Backups you make
            yourself are kept until you delete them. They're stored in{' '}
            <span className="break-all font-mono">{backups.data.folder}</span>.
          </p>
        )}
      </div>

      <ConfirmDialog
        open={!!restoring}
        onOpenChange={(o) => {
          if (!o) {
            setRestoring(null);
            restore.reset();
          }
        }}
        title="Restore this backup?"
        confirmLabel="Restore"
        busy={restore.isPending}
        error={restore.error?.message}
        onConfirm={() => restoring && restore.mutate(restoring)}
      >
        <p>
          Arc will go back to how it was on <span className="text-foreground">{fmt.dateTime(restoring?.created_at)}</span>:
          matches, episode orders and everything else it knows. Changes made since then will be undone.
        </p>
        <p>Your current state is saved as a backup first, so you can switch back. Your media files aren't touched.</p>
      </ConfirmDialog>

      <ConfirmDialog
        open={!!deleting}
        onOpenChange={(o) => {
          if (!o) {
            setDeleting(null);
            remove.reset();
          }
        }}
        title="Delete this backup?"
        confirmLabel="Delete"
        destructive
        busy={remove.isPending}
        error={remove.error?.message}
        onConfirm={() => deleting && remove.mutate(deleting)}
      >
        <p>The backup from {fmt.dateTime(deleting?.created_at)} will be permanently deleted.</p>
      </ConfirmDialog>
    </Page>
  );
}

function BackupRow({ backup, latest, onRestore, onDelete }: { backup: Backup; latest: boolean; onRestore: () => void; onDelete: () => void }) {
  const { label, icon: Icon } = reasons[backup.reason] ?? reasons.manual;
  return (
    <div className="flex items-center gap-4 px-4 py-3.5">
      <div className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-surface-2 text-muted">
        <Icon className="size-[18px]" />
      </div>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium">
          {fmt.dateTime(backup.created_at)}
          {latest && <span className="ml-2 rounded-full bg-accent/15 px-2 py-px text-[11px] font-semibold text-accent">Latest</span>}
        </p>
        <p className="truncate text-xs text-subtle">
          {label} · {fmt.fileSize(backup.size)}
        </p>
      </div>
      <Button variant="secondary" size="sm" onClick={onRestore}>
        <ArchiveRestore /> Restore
      </Button>
      <Button variant="ghost" size="icon" onClick={onDelete} title="Delete backup" aria-label="Delete backup">
        <Trash2 />
      </Button>
    </div>
  );
}
