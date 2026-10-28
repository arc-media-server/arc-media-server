import { useTranslation } from 'react-i18next';
import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Trash2 } from 'lucide-react';
import { api } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';

export function EmptyTrashDialog({ libraryId, libraryName, open, onOpenChange }: {
  libraryId: number; libraryName: string; open: boolean; onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslation('serverSettings');
  const qc = useQueryClient();
  const empty = useMutation({ mutationFn: () => api.emptyTrash(libraryId), onSuccess: () => {
    onOpenChange(false);
    void qc.invalidateQueries();
  } });
  return <ConfirmDialog open={open} onOpenChange={o => {
    if (empty.isPending) return;
    onOpenChange(o);
    if (!o) empty.reset();
  }} title={t("serverSettings:emptyTrash.removeTitlesWhoseFiles")} confirmLabel={t("serverSettings:emptyTrash.remove")} destructive busy={empty.isPending}
    error={empty.error?.message} onConfirm={() => empty.mutate()}>
    <p>{t('serverSettings:emptyTrash.explanation', { name: libraryName })}</p>
    <p>{t("serverSettings:emptyTrash.onlyDoThisIf")}</p>
  </ConfirmDialog>;
}

export function EmptyTrashButton({ libraryId, libraryName }: { libraryId: number; libraryName: string }) {
  const { t } = useTranslation('serverSettings');
  const [open, setOpen] = useState(false);
  return <><Button variant="secondary" size="sm" className="shrink-0" onClick={() => setOpen(true)}><Trash2 /> {t("serverSettings:emptyTrash.removeAll")}</Button>
    <EmptyTrashDialog libraryId={libraryId} libraryName={libraryName} open={open} onOpenChange={setOpen} /></>;
}
