import { Trans, useTranslation } from 'react-i18next';
import { useState } from 'react';
import { Link } from 'react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArchiveRestore, CheckCircle2, DatabaseBackup, Hand, History, Loader2, Plus, ShieldCheck, Trash2 } from 'lucide-react';
import { api, type Backup, type BackupReason } from '@/lib/api';
import * as fmt from '@/lib/format';
import { EmptyState, ErrorNotice, ErrorState, Page, PageHeader } from '@/components/Page';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';



export function BackupsPage() {
  const { t } = useTranslation('administration');
  const qc = useQueryClient();
  const backups = useQuery({ queryKey: ['backups'], queryFn: api.backups });
  const settings = useQuery({ queryKey: ['settings'], queryFn: api.settings });
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
        title={t("administration:backups.backups")}
        subtitle={t("administration:backups.backUpAndRestore")}
        actions={
          <Button variant="secondary" size="sm" onClick={() => create.mutate()} disabled={create.isPending}>
            {create.isPending ? <Loader2 className="animate-spin" /> : <Plus />} {t("administration:backups.backUpNow")}</Button>
        }
      />
      <div className="space-y-4 px-6 md:px-10">
        <section aria-label={t("administration:backups.backupScope")} className="space-y-2 rounded-2xl bg-surface p-4 text-sm ring-1 ring-border">
          <h2 className="font-medium">{t("administration:backups.databaseBackupsAndFull")}</h2>
          <p className="text-muted">{t("administration:backups.backUpNowSaves")}</p>
          <p className="text-muted">{t("administration:backups.forAFullServer")}</p>
          <p className="text-muted">{t("administration:backups.restoringADatabaseBackup")}</p>
        </section>
        <section aria-label={t("administration:backups.automaticBackupStatus")} className="space-y-2 rounded-2xl bg-surface p-4 ring-1 ring-border text-sm">
          {settings.error ? (
            <>
              <p>{t("administration:backups.automaticBackupStatusIs")}</p>
              <ErrorNotice error={settings.error} onRetry={() => void settings.refetch()} busy={settings.isFetching} />
            </>
          ) : !settings.data ? (
            <p role="status">{t("administration:backups.loadingAutomaticBackupStatus")}</p>
          ) : (
            <>
              <p className="font-medium">{settings.data.auto_backups_enabled ? t('administration:backups.automaticOn') : t('administration:backups.automaticOff')}</p>
              {settings.data.auto_backups_enabled ? (
                <p className="text-muted">{settings.data.maintenance_start === settings.data.maintenance_end ? t('administration:backups.allDaySchedule') : t('administration:backups.windowSchedule', { window: `${String(settings.data.maintenance_start).padStart(2, '0')}:00–${String(settings.data.maintenance_end).padStart(2, '0')}:00` })}</p>
              ) : <p className="text-muted">{t("administration:backups.arcIsNotTaking")}</p>}
            </>
          )}
          <p className="text-muted">{t("administration:backups.backUpNowAnd")}</p>
          {backups.data && <p className="text-muted">{backups.data.backups[0]
            ? t('administration:backups.lastSuccess', { date: fmt.dateTime(backups.data.backups[0].created_at) })
            : t("administration:backups.noSuccessfulBackupsAre")}</p>}
          <Link to="/settings?section=maintenance#scheduled-tasks" className="inline-block text-accent hover:underline">{t("administration:backups.manageAutomaticBackups")}</Link>
        </section>
        {restored && (
          <p className="flex items-center gap-2 rounded-2xl bg-accent/10 px-4 py-3 text-sm ring-1 ring-accent/25">
            <CheckCircle2 className="size-4 shrink-0 text-accent" />{t('administration:backups.restoredNotice', { date: fmt.dateTime(restored.created_at) })}</p>
        )}
        {create.error && <p className="text-sm text-warning">{t('administration:backups.backupError', { error: create.error.message })}</p>}

        {backups.data && backups.data.backups.length === 0 ? (
          <EmptyState icon={<DatabaseBackup />} title={t("administration:backups.noBackupsYet")}>{t("administration:backups.useBackUpNow")}</EmptyState>
        ) : (
          <div className="divide-y divide-border overflow-hidden rounded-2xl bg-surface ring-1 ring-border">
            {!backups.data && <div className="h-40 animate-pulse" />}
            {backups.data?.backups.map((b, i) => (
              <BackupRow key={b.name} backup={b} latest={i === 0} onRestore={() => setRestoring(b)} onDelete={() => setDeleting(b)} />
            ))}
          </div>
        )}

        {backups.data && (
          <p className="max-w-3xl text-xs leading-relaxed text-subtle"><Trans ns="administration" i18nKey="backups.folderDescription" values={{ folder: backups.data.folder }} components={{ folder: <span className="break-all font-mono" /> }} />
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
        title={t("administration:backups.restoreThisBackup")}
        confirmLabel={t("administration:backups.restore")}
        busy={restore.isPending}
        error={restore.error?.message}
        onConfirm={() => restoring && restore.mutate(restoring)}
      >
        <p><Trans ns="administration" i18nKey="backups.restoreDescription" values={{ date: fmt.dateTime(restoring?.created_at) }} components={{ date: <span className="text-foreground" /> }} /></p>
        <p>{t("administration:backups.yourCurrentDatabaseIs")}</p>
      </ConfirmDialog>

      <ConfirmDialog
        open={!!deleting}
        onOpenChange={(o) => {
          if (!o) {
            setDeleting(null);
            remove.reset();
          }
        }}
        title={t("administration:backups.deleteThisBackup")}
        confirmLabel={t("administration:backups.delete")}
        destructive
        busy={remove.isPending}
        error={remove.error?.message}
        onConfirm={() => deleting && remove.mutate(deleting)}
      >
        <p>{t('administration:backups.deleteDescription', { date: fmt.dateTime(deleting?.created_at) })}</p>
      </ConfirmDialog>
    </Page>
  );
}

function BackupRow({ backup, latest, onRestore, onDelete }: { backup: Backup; latest: boolean; onRestore: () => void; onDelete: () => void }) {
  const { t } = useTranslation('administration');
  const reasons: Record<BackupReason, { label: string; icon: typeof History }> = {
  auto: { label: t('administration:reasons.daily'), icon: History },
  manual: { label: t('administration:reasons.manual'), icon: Hand },
  'before-restore': { label: t('administration:reasons.beforeRestore'), icon: ArchiveRestore },
  'before-upgrade': { label: t('administration:reasons.beforeUpdate'), icon: ShieldCheck },
  'before-delete': { label: t('administration:reasons.beforeDelete'), icon: ShieldCheck },
};
  const { label, icon: Icon } = reasons[backup.reason] ?? reasons.manual;
  return (
    <div className="flex items-center gap-4 px-4 py-3.5">
      <div className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-surface-2 text-muted">
        <Icon className="size-[18px]" />
      </div>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium">
          {fmt.dateTime(backup.created_at)}
          {latest && <span className="ml-2 rounded-full bg-accent/15 px-2 py-px text-[11px] font-semibold text-accent">{t("administration:backups.latest")}</span>}
        </p>
        <p className="truncate text-xs text-subtle">
          {label} · {fmt.fileSize(backup.size)}
        </p>
      </div>
      <Button variant="secondary" size="sm" onClick={onRestore}>
        <ArchiveRestore /> {t("administration:backups.restore")}</Button>
      <Button variant="ghost" size="icon" onClick={onDelete} title={t("administration:backups.deleteBackup")} aria-label={t("administration:backups.deleteBackup")}>
        <Trash2 />
      </Button>
    </div>
  );
}
